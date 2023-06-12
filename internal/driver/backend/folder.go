package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
)

type httpFolderResponse struct {
	ID        string `json:"id"`
	LocalPath string `json:"local_path"`
}

func bodyToError(resp *http.Response) error {
	var errMessage bytes.Buffer
	errMessage.WriteString("HTTP Status ")
	errMessage.WriteString(resp.Status)
	if resp.Body != nil {
		errText, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if err != nil {
			errMessage.WriteString(", Error: ")
			errMessage.WriteString(err.Error())
		} else {
			errMessage.WriteString(", Response: ")
			errMessage.Write(errText)
		}
	}
	return errors.New(errMessage.String())
}

// httpFolder returns the folder to watch
// BEWARE: backoff is not thread-safe, do not share amongst goroutines
func (s *Server) httpFolder(ctx context.Context, bo backoff.BackOff, authChan chan<- AuthRequest) (string, error) {
	logger := s.auth.logger
	// Build the request
	parsedURL, err := url.Parse(s.auth.apiURL + "/api/camera/" + url.PathEscape(s.cameraID))
	if err != nil {
		logger.Error("failed to parse auth url", servicelog.Error(err))
		return "", err
	}
	reqURL := parsedURL.String()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		logger.Error("failed to build request", servicelog.Error(err))
		return "", err
	}
	var folder string
	err = backoff.Retry(func() (returnErr error) {
		defer func() {
			returnErr = PermanentIfCancel(ctx, returnErr)
		}()
		resp, err := s.auth.Do(ctx, req, authChan)
		if resp != nil {
			defer exhaust(resp.Body)
		}
		if err != nil {
			logger.Error("failed to get folder", servicelog.Error(err))
			return err
		}
		if resp.Body == nil {
			logger.Error("empty response body")
			return EmptyFolderResponseError
		}
		if resp.StatusCode != http.StatusOK {
			err = bodyToError(resp)
			logger.Error("failed to get folder", servicelog.Error(err))
			return err
		}
		var folderResponse httpFolderResponse
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(&folderResponse); err != nil {
			logger.Error("failed to decode response", servicelog.Error(err))
			return err
		}
		folder = folderResponse.LocalPath
		return nil
	}, backoff.WithContext(bo, ctx))
	bo.Reset()
	return folder, err
}

// WatchFolder watches the folder for changes and notifies them in the folderChan
func (s *Server) WatchFolder(ctx context.Context, authChan chan<- AuthRequest, folderChan chan<- string, interval time.Duration) {
	bo := eternalBackoff()
	logger := s.auth.logger
	timer := time.NewTimer(interval)
	var lastFolder string
	for {
		folder, err := s.httpFolder(ctx, bo, authChan)
		if err != nil {
			logger.Error("failed to get folder", servicelog.Error(err))
			continue
		}
		if folder != lastFolder {
			select {
			case <-ctx.Done():
				return
			case folderChan <- folder:
				lastFolder = folder
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			timer.Reset(interval)
		}
	}
}
