package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
)

// httpMediaRequest implements the Resource interface for media
// (pictures and fotos)
type httpMediaRequest struct {
	ID        string       `json:"id"`
	Timestamp string       `json:"timestamp"`
	Camera    string       `json:"camera"`
	Tags      []string     `json:"tags,omitempty"`
	MediaType string       `json:"-"` // picture or video
	MimeType  string       `json:"-"`
	Buffer    bytes.Buffer `json:"-"`
}

// PostURL implements resource
func (hmr httpMediaRequest) PostURL(apiURL string) string {
	return fmt.Sprintf("%s/api/%s", apiURL, hmr.MediaType)
}

// PostBody implements resource
func (hmr httpMediaRequest) PostBody() (io.ReadCloser, error) {
	if hmr.Buffer.Len() == 0 {
		encoder := json.NewEncoder(&hmr.Buffer)
		if err := encoder.Encode(hmr); err != nil {
			return nil, err
		}
	}
	return ioutil.NopCloser(bytes.NewBuffer(hmr.Buffer.Bytes())), nil
}

// PostType implements resource
func (hmr httpMediaRequest) PostType() string {
	return "application/json"
}

// PutURL implements resource
func (hmr httpMediaRequest) PutURL(apiURL string) string {
	return fmt.Sprintf("%s/api/%s/%s", apiURL, hmr.MediaType, url.PathEscape(hmr.ID))
}

// PutBody implements resource
func (hmr httpMediaRequest) PutBody() (io.ReadCloser, error) {
	return hmr.PostBody()
}

// Media sends a media resource to the server
func (s *Server) Media(ctx context.Context, authChan chan<- AuthRequest, mimeType string, path string) error {
	logger := s.logger.With(servicelog.String("path", path), servicelog.String("mimeType", mimeType))
	id := fmt.Sprintf("%s_%s", s.cameraID, filepath.Base(path))
	var mediaType string
	if strings.HasPrefix(mimeType, "video") {
		mediaType = "video"
	}
	if strings.HasPrefix(mimeType, "image") {
		mediaType = "picture"
	}
	if mediaType == "" {
		logger.Error("failed to detect media type")
		return UnknownMediaTypeError
	}
	// Limit concurrent uploads to the server, to preserve BW
	logger.Debug("getting concurrency token")
	<-s.queue
	defer func() {
		logger.Debug("going to release concurrency token")
		s.queue <- struct{}{}
		logger.Debug("concurrency token released")
	}()
	logger.Debug("got concurrency token")
	info, err := os.Stat(path)
	if err != nil {
		logger.Error("failed to stat media file", servicelog.Error(err))
		return err
	}
	media := httpMediaRequest{
		ID:        id,
		Timestamp: info.ModTime().UTC().Format(time.RFC3339),
		Camera:    s.cameraID,
		Tags:      []string{"automatic"},
		MediaType: mediaType,
		MimeType:  mimeType,
	}
	err = s.sendResource(ctx, authChan, media, sendOptions{
		maxRetries: 3,
	})
	if err != nil {
		logger.Error("failed to send media metadata")
	} else {
		// post file body
		logger.Info("sending media contents")
		fileReq := &httpFileRequest{
			ID:        id,
			Path:      path,
			MediaType: mediaType,
			MimeType:  mimeType,
			Logger:    logger,
		}
		err = s.sendResource(ctx, authChan, fileReq, sendOptions{
			maxRetries: 3,
			onlyPost:   true,
		})
		if err == nil {
			logger.Debug("done sending media contents")
		} else {
			logger.Error("failed to send media contents", servicelog.Error(err))
		}
	}
	return err
}
