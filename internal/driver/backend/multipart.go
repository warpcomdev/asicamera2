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
)

type httpFileRequest struct {
	Mutex     sync.Mutex `json:"-"` // protects the errors
	ID        string     `json:"id"`
	Path      string     `json:"path"`
	MediaType string     `json:"mediaType"`
	MimeType  string     `json:"mimeType"`
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
	if hfr.Stop != nil {
		close(hfr.Stop)
		hfr.WG.Wait()
	}
	hfr.Mutex.Lock()
	defer hfr.Mutex.Unlock()
	err := hfr.CloseError
	// LEave the struct in a consistent state
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
		// Copied from CreateFormFile
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes("file"), escapeQuotes(hfr.Path)))
		h.Set("Content-Type", hfr.MimeType)
		w, err := mwriter.CreatePart(h)
		if err != nil {
			return err
		}
		in, err := os.Open(hfr.Path)
		if err != nil {
			return err
		}
		defer in.Close()
		controlledIn := controlledReader{
			reader: in,
			stop:   stopper,
		}
		if written, err := io.Copy(w, controlledIn); err != nil {
			return fmt.Errorf("error after copying %d bytes: %w", written, err)
		}
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
	return hfr.MultipartWriter.FormDataContentType()
}

// PutURL implements resource
func (hfr *httpFileRequest) PutURL(apiURL string) string {
	return ""
}

// PutBody implements resource
func (hfr *httpFileRequest) PutBody() (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}
