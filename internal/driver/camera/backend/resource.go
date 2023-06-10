package backend

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/cenkalti/backoff"
	"go.uber.org/zap"
)

// validateURL validates a URL and returns it if valid
func validateURL(origURL string) (string, error) {
	if origURL == "" {
		return origURL, nil
	}
	parsedURL, err := url.Parse(origURL)
	if err != nil {
		return "", err
	}
	return parsedURL.String(), nil
}

// Resource that can be Put or Posted
type resource interface {
	PostURL(apiURL string) string
	PostBody() (io.ReadCloser, error)
	PostType() string
	PutURL(apiURL string) string
	PutBody() (io.ReadCloser, error)
}

func (s *Server) sendResource(ctx context.Context, authChan chan<- AuthRequest, resource resource, maxRetries int, limitConcurrency bool) error {
	logger := s.auth.logger
	// Build the request for POST
	postURL, err := validateURL(resource.PostURL(s.auth.apiURL))
	if err != nil {
		logger.Error("failed to validate post url", zap.Error(err))
		return err
	}
	logger = logger.With(zap.String("postURL", postURL))
	// Build the request for PUT
	var bo backoff.BackOff = backoff.NewExponentialBackOff()
	if maxRetries > 0 {
		bo = backoff.WithMaxRetries(bo, uint64(maxRetries))
	}
	err = backoff.Retry(func() (returnErr error) {
		defer func() {
			returnErr = PermanentIfCancel(ctx, returnErr)
		}()
		// If concurrency of this resource is controller, pick an item from the queue
		// so only as many as the number of concurrent uploads is in transit
		if limitConcurrency {
			<-s.queue
			defer func() {
				s.queue <- struct{}{}
			}()
		}
		// Build the request.
		postBody, err := resource.PostBody()
		if err != nil {
			logger.Error("failed to build request body", zap.Error(err))
			return &backoff.PermanentError{Err: err}
		}
		defer postBody.Close()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, postURL, postBody)
		if err != nil {
			logger.Error("failed to build request", zap.Error(err))
			return &backoff.PermanentError{Err: err}
		}
		req.Header.Set("Content-Type", resource.PostType())
		resp, err := s.auth.Do(ctx, req, authChan)
		if resp != nil {
			defer exhaust(resp.Body)
		}
		if err != nil {
			logger.Error("failed to post data", zap.Error(err))
			return err
		}
		// If POST failed, try PUT
		if resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusInternalServerError {
			putURL, err := validateURL(resource.PutURL(s.auth.apiURL))
			if err != nil {
				logger.Error("failed to validate put url", zap.Error(err))
				return &backoff.PermanentError{Err: err}
			}
			if putURL == "" {
				return &backoff.PermanentError{Err: PostFailedError}
			}
			putBody, err := resource.PutBody()
			if err != nil {
				logger.Error("failed to build request body", zap.Error(err))
				return &backoff.PermanentError{Err: err}
			}
			defer putBody.Close()
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL, putBody)
			if err != nil {
				logger.Error("failed to build request", zap.Error(err))
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err = s.auth.Do(ctx, req, authChan)
			if resp != nil {
				defer exhaust(resp.Body)
			}
			if err != nil {
				logger.Error("failed to put resource", zap.Error(err))
				return err
			}
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
			err = bodyToError(resp)
			logger.Error("failed to put data", zap.Error(err))
			return err
		}
		return nil
	}, bo)
	bo.Reset()
	return err
}
