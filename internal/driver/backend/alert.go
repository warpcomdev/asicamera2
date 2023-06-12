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

type Alert struct {
	ID         string `json:"id"`
	Timestamp  string `json:"timestamp"`
	Name       string `json:"name,omitempty"`
	Camera     string `json:"camera,omitempty"`
	Severity   string `json:"severity,omitempty"`
	Message    string `json:"message,omitempty"`
	ResolvedAt string `json:"resolved_at,omitempty"`
}

type httpAlertRequest struct {
	Alert
	Buffer bytes.Buffer `json:"-"`
}

// PostURL implements resource
func (har httpAlertRequest) PostURL(apiURL string) string {
	return fmt.Sprintf("%s/api/alert", apiURL)
}

// PostBody implements resource
func (har httpAlertRequest) PostBody() (io.ReadCloser, error) {
	if har.Buffer.Len() == 0 {
		encoder := json.NewEncoder(&har.Buffer)
		if err := encoder.Encode(har.Alert); err != nil {
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

func (s *Server) SendAlert(ctx context.Context, authChan chan<- AuthRequest, id, name, severity, message string) {
	alert := httpAlertRequest{
		Alert: Alert{
			ID:        id,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Name:      name,
			Camera:    s.cameraID,
			Severity:  severity,
			Message:   message,
		},
	}
	s.sendResource(ctx, authChan, alert, sendOptions{onlyPost: true, maxRetries: 3})
}

func (s *Server) ClearAlert(ctx context.Context, authChan chan<- AuthRequest, id string) {
	now := time.Now().UTC().Format(time.RFC3339)
	alert := httpAlertRequest{
		Alert: Alert{
			ID:         id,
			Timestamp:  now,
			ResolvedAt: now,
		},
	}
	s.sendResource(ctx, authChan, alert, sendOptions{onlyPut: true, maxRetries: 3})
}
