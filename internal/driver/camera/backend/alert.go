package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"time"
)

type httpAlertRequest struct {
	ID        string       `json:"id"`
	Timestamp string       `json:"timestamp"`
	Camera    string       `json:"camera"`
	Severity  string       `json:"severity"`
	Message   string       `json:"message"`
	Buffer    bytes.Buffer `json:"-"`
}

// PostURL implements resource
func (har httpAlertRequest) PostURL(apiURL string) string {
	return fmt.Sprintf("%s/api/alert", apiURL)
}

// PostBody implements resource
func (har httpAlertRequest) PostBody() (io.ReadCloser, error) {
	if har.Buffer.Len() == 0 {
		encoder := json.NewEncoder(&har.Buffer)
		if err := encoder.Encode(har); err != nil {
			return nil, err
		}
	}
	return ioutil.NopCloser(bytes.NewBuffer(har.Buffer.Bytes())), nil
}

// PostType implements resource
func (har httpAlertRequest) PostType() string {
	return "application/json"
}

// PutURL implements resource
func (har httpAlertRequest) PutURL(apiURL string) string {
	return fmt.Sprintf("%s/api/alert/%s", apiURL, url.PathEscape(har.ID))
}

// PutBody implements resource
func (har httpAlertRequest) PutBody() (io.ReadCloser, error) {
	return har.PostBody()
}

func (s *Server) Alert(ctx context.Context, authChan chan<- AuthRequest, id, severity, message string) {
	alert := httpAlertRequest{
		ID:        id,
		Timestamp: time.Now().Format(time.RFC3339),
		Camera:    s.cameraID,
		Severity:  severity,
		Message:   message,
	}
	s.sendResource(ctx, authChan, alert, -1, false)
}
