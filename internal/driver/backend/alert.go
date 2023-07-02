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

	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
)

// Alert implements the alert resource
type Alert struct {
	ID         string `json:"id"`
	Timestamp  string `json:"timestamp"`
	Name       string `json:"name,omitempty"`
	Camera     string `json:"camera,omitempty"`
	Severity   string `json:"severity,omitempty"`
	Message    string `json:"message,omitempty"`
	ResolvedAt string `json:"resolved_at,omitempty"`
}

type alertResponse struct {
	Data []interface{} `json:"data"`
	Next string        `json:"next"`
}

type httpAlertRequest struct {
	Alert
	Buffer bytes.Buffer `json:"-"`
	// In case this object is used for get request
	Response alertResponse `json:"-"`
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

// GetURL implements getResource
func (har httpAlertRequest) GetURL(apiURL string) string {
	return fmt.Sprintf("%s/api/alert?q:id:eq=%s", apiURL, url.QueryEscape(har.ID))
}

// ReadBody implements getResource
func (har httpAlertRequest) ReadBody(body io.Reader) error {
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&har.Response); err != nil {
		return err
	}
	return nil
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

// Clear an alert if it exists
func (s *Server) ClearAlert(ctx context.Context, authChan chan<- AuthRequest, id string) {
	now := time.Now().UTC().Format(time.RFC3339)
	alert := httpAlertRequest{
		Alert: Alert{
			ID:         id,
			Timestamp:  now,
			ResolvedAt: now,
		},
	}
	if err := s.getResource(ctx, authChan, alert, sendOptions{maxRetries: 3}); err != nil {
		logger := s.logger.With(servicelog.String("id", id))
		logger.Error("failed to get alert status", servicelog.Error(err))
	} else {
		if alert.Response.Data != nil && len(alert.Response.Data) > 0 {
			s.sendResource(ctx, authChan, alert, sendOptions{onlyPut: true, maxRetries: 3})
		}
	}
}
