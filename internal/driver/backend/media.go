package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

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
	id := fmt.Sprintf("%s_%s", s.cameraID, filepath.Base(path))
	var mediaType string
	if strings.HasPrefix(mimeType, "video") {
		mediaType = "video"
	}
	if strings.HasPrefix(mimeType, "image") {
		mediaType = "picture"
	}
	if mediaType == "" {
		return UnknownMediaTypeError
	}
	media := httpMediaRequest{
		ID:        id,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Camera:    s.cameraID,
		Tags:      []string{"automatic"},
		MediaType: mediaType,
		MimeType:  mimeType,
	}
	err := s.sendResource(ctx, authChan, media, sendOptions{
		maxRetries: 3,
	})
	if err == nil {
		// post file body
		file := &httpFileRequest{
			ID:        id,
			Path:      path,
			MediaType: mediaType,
			MimeType:  mimeType,
		}
		err = s.sendResource(ctx, authChan, file, sendOptions{
			maxRetries:       3,
			limitConcurrency: true,
		})
	}
	return err
}
