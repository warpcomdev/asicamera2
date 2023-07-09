package backend

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
)

var MediaTransferCount = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "media_transferred_count",
		Help: "Number of Media (picture and video) files transferred",
	},
	[]string{"mimetype"},
)

var MediaTransferError = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "media_transferred_errors",
		Help: "Number of Media (picture and video) files failed to transfer",
	},
	[]string{"mimetype"},
)

var MediaTransferBytes = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "media_transferred_bytes",
		Help: "Media (picture and video) bytes transferred",
	},
	[]string{"mimetype"},
)

var MediaTransferBytesError = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "media_transferred_bytes_error",
		Help: "Media (picture and video) bytes transferred before returning error",
	},
	[]string{"mimetype"},
)

var MediaTransferTime = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "media_transferred_seconds",
		Help: "Transfer time for files (seconds)",
		Buckets: []float64{
			1, 5, 10, 30, 60, 180, 600, 1800,
		},
	},
	[]string{"mimetype"},
)

var MediaFileSize = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "media_file_size",
		Help: "media file sizes (bytes)",
		Buckets: []float64{
			// These sizes are intended for pictures
			512 * 1024,
			1024 * 1024,
			4 * 1024 * 1024,
			16 * 1024 * 1024,
			32 * 1024 * 1024,
			// Those are intende dfor video
			128 * 1024 * 1024,
			512 * 1024 * 1024,
			1024 * 1024 * 1024,
			2 * 1024 * 1024 * 1024,
			4 * 1024 * 1024 * 1024,
			8 * 1024 * 1024 * 1024,
		},
	},
	[]string{"mimetype"},
)

// httpFileRequest implements the Resource interface for media content
// (multipart body with file contents)
type httpFileRequest struct {
	Mutex     sync.Mutex        `json:"-"` // protects the errors
	ID        string            `json:"id"`
	Path      string            `json:"path"`
	MediaType string            `json:"mediaType"`
	MimeType  string            `json:"mimeType"`
	Logger    servicelog.Logger `json:"-"`
	// Controls the lifetime of the pipe reader
	PipeReader      io.ReadCloser     `json:"-"`
	MultipartWriter *multipart.Writer `json:"-"`
	Stop            chan struct{}     `json:"-"`
	WG              *sync.WaitGroup   `json:"-"`
	// Reports errors while reading
	ReadError  error `json:"-"`
	CloseError error `json:"-"`
}

// Read implements ReadCloser
func (hfr *httpFileRequest) Read(b []byte) (int, error) {
	n, err := hfr.PipeReader.Read(b)
	hfr.Logger.Debug("read from disk", servicelog.Int("bytes", n))
	// Propagate errors, if any
	hfr.Mutex.Lock()
	defer hfr.Mutex.Unlock()
	// Beware, we cannot unconditionally use errors.Join,
	// because we want to preserve io.EOF transparently
	if hfr.ReadError != nil {
		return n, errors.Join(err, hfr.ReadError)
	}
	return n, err
}

// Close implements ReadCloser
func (hfr *httpFileRequest) Close() error {
	// Stop the reader if it's running
	hfr.Logger.Debug("closing multipart transfer")
	if hfr.Stop != nil {
		close(hfr.Stop)
		hfr.WG.Wait()
	}
	hfr.Mutex.Lock()
	defer hfr.Mutex.Unlock()
	err := hfr.CloseError
	// Leave the struct in a consistent state
	hfr.Stop = nil
	hfr.WG = nil
	hfr.MultipartWriter = nil
	hfr.PipeReader = nil
	hfr.ReadError = nil
	hfr.CloseError = nil
	return err
}

// PostURL implements resource
func (hfr *httpFileRequest) PostURL(apiURL string) string {
	return apiURL + "/api/" + hfr.MediaType + "/" + hfr.ID
}

// ControlledReader returns a reader that can be stopped
type controlledReader struct {
	reader io.Reader
	stop   chan struct{}
}

func (r controlledReader) Read(p []byte) (n int, err error) {
	select {
	case <-r.stop:
		return 0, io.EOF
	default:
		return r.reader.Read(p)
	}
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// PostBody implements resource
func (hfr *httpFileRequest) PostBody() (io.ReadCloser, error) {
	reader, writer := io.Pipe()
	mwriter := multipart.NewWriter(writer)
	stopper := make(chan struct{})
	hfr.ReadError = nil
	hfr.CloseError = nil
	wg := &sync.WaitGroup{}
	wg.Add(1)
	// The returnErr in this closure is captured by a defer
	// inside it, and saved into the struct
	go func() (returnErr error) {
		defer wg.Done()
		// Merge all possible errors into one
		defer func() {
			hfr.Mutex.Lock()
			defer hfr.Mutex.Unlock()
			if returnErr != nil && !errors.Is(returnErr, io.EOF) {
				hfr.ReadError = returnErr
			}
			merr := mwriter.Close()
			werr := writer.Close()
			hfr.CloseError = errors.Join(merr, werr)
		}()
		hfr.Logger.Debug("multipart transfer started")
		// Copied from CreateFormFile
		start := time.Now()
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes("file"), escapeQuotes(hfr.Path)))
		h.Set("Content-Type", hfr.MimeType)
		w, err := mwriter.CreatePart(h)
		if err != nil {
			hfr.Logger.Error("failed to create multipart writer", servicelog.Error(err))
			return err
		}
		in, err := os.Open(hfr.Path)
		if err != nil {
			hfr.Logger.Error("failed to open source file", servicelog.Error(err))
			return err
		}
		defer in.Close()
		controlledIn := controlledReader{
			reader: in,
			stop:   stopper,
		}
		written, err := io.Copy(w, controlledIn)
		if err != nil {
			hfr.Logger.Error("failed to copy file contents", servicelog.Error(err))
			MediaTransferError.WithLabelValues(hfr.MimeType).Add(1)
			MediaTransferBytesError.WithLabelValues(hfr.MimeType).Add(float64(written))
			return fmt.Errorf("error after copying %d bytes: %w", written, err)
		}
		MediaTransferTime.WithLabelValues(hfr.MimeType).Observe(float64(time.Since(start) / time.Second))
		MediaTransferCount.WithLabelValues(hfr.MimeType).Add(1)
		MediaTransferBytes.WithLabelValues(hfr.MimeType).Add(float64(written))
		MediaFileSize.WithLabelValues(hfr.MimeType).Observe(float64(written))
		hfr.Logger.Debug("multipart transfer finished")
		return nil
	}()
	hfr.PipeReader = reader
	hfr.MultipartWriter = mwriter
	hfr.Stop = stopper
	hfr.WG = wg
	return hfr, nil
}

// PostType implements resource
func (hfr *httpFileRequest) PostType() string {
	postType := hfr.MultipartWriter.FormDataContentType()
	hfr.Logger.Debug("multipart content type", servicelog.String("content-type", postType))
	return postType
}

// PutURL implements resource
func (hfr *httpFileRequest) PutURL(apiURL string) string {
	return ""
}

// PutBody implements resource
func (hfr *httpFileRequest) PutBody() (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}
