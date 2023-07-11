package backend

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
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
	// URL for POSTing (creating) this resource
	PostURL(apiURL string) string
	// Body for POSTing (creating) this resource
	PostBody() (io.ReadCloser, error)
	// Content-Type for POSTing (creating) this resource
	PostType() string
	// URL for PUTting (updating) this resource
	PutURL(apiURL string) string
	// Body for PUTting (updating) this resource
	PutBody() (io.ReadCloser, error)
}

// Resource that can be Put or Posted
type getResource interface {
	// URL for POSTing (creating) this resource
	GetURL(apiURL string) string
	ReadBody(body io.Reader) error
}

func eternalBackoff() backoff.BackOff {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 1 * time.Second
	bo.Multiplier = 2
	bo.MaxInterval = 5 * time.Minute
	bo.MaxElapsedTime = 0
	return bo
}

type sendOptions struct {
	maxRetries int
	onlyPut    bool
	onlyPost   bool
}

func (s *Server) sendResource(ctx context.Context, authChan chan<- AuthRequest, resource resource, opts sendOptions) error {
	logger := s.auth.logger
	// Build the request for POST
	postURL, err := validateURL(resource.PostURL(s.auth.apiURL))
	if err != nil {
		logger.Error("failed to validate post url", servicelog.Error(err))
		return err
	}
	logger = logger.With(servicelog.String("postURL", postURL))
	// Build the request for PUT
	var bo backoff.BackOff = eternalBackoff()
	if opts.maxRetries > 0 {
		bo = backoff.WithMaxRetries(bo, uint64(opts.maxRetries))
	}
	err = backoff.Retry(func() (returnErr error) {
		defer func() {
			returnErr = PermanentIfCancel(ctx, returnErr)
		}()
		var resp *http.Response
		if !opts.onlyPut {
			// Build the request.
			postBody, err := resource.PostBody()
			if err != nil {
				logger.Error("failed to build request body", servicelog.Error(err))
				return &backoff.PermanentError{Err: err}
			}
			defer postBody.Close()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, postURL, postBody)
			if err != nil {
				logger.Error("failed to build request", servicelog.Error(err))
				return &backoff.PermanentError{Err: err}
			}
			req.Header.Set("Content-Type", resource.PostType())
			resp, err = s.auth.Do(ctx, req, authChan)
			if resp != nil {
				defer exhaust(resp.Body)
			}
			if err != nil {
				logger.Error("failed to post data", servicelog.Error(err))
				return err
			}
		}
		// If only doing PUT, or POST failed and not only doing POST, try PUT
		if !opts.onlyPost && (opts.onlyPut || resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusInternalServerError) {
			// This exhausts the POST body, so the connection is not busy
			if resp != nil {
				postErr := bodyToError(resp)
				logger.Debug("POST failed, trying PUT", servicelog.Error(postErr))
			}
			putURL, err := validateURL(resource.PutURL(s.auth.apiURL))
			if err != nil {
				logger.Error("failed to validate put url", servicelog.Error(err))
				return &backoff.PermanentError{Err: err}
			}
			if putURL == "" {
				logger.Debug("no put url, skipping put")
				return &backoff.PermanentError{Err: PostFailedError}
			}
			putBody, err := resource.PutBody()
			if err != nil {
				logger.Error("failed to build request body", servicelog.Error(err))
				return &backoff.PermanentError{Err: err}
			}
			defer putBody.Close()
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL, putBody)
			if err != nil {
				logger.Error("failed to build request", servicelog.Error(err))
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err = s.auth.Do(ctx, req, authChan)
			if resp != nil {
				defer exhaust(resp.Body)
			}
			if err != nil {
				logger.Error("failed to put resource", servicelog.Error(err))
				return err
			}
		}
		if resp == nil {
			logger.Error("could not decide to POST or PUT", servicelog.Bool("onlyPost", opts.onlyPost), servicelog.Bool("onlyPut", opts.onlyPut))
		}
		if resp != nil && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
			err = bodyToError(resp)
			logger.Error("failed to upsert data", servicelog.Error(err))
			return err
		}
		logger.Debug("resource send complete")
		return nil
	}, backoff.WithContext(bo, ctx))
	bo.Reset()
	return err
}

func (s *Server) getResource(ctx context.Context, authChan chan<- AuthRequest, resource getResource, opts sendOptions) error {
	logger := s.auth.logger
	// Build the request for POST
	getURL, err := validateURL(resource.GetURL(s.auth.apiURL))
	if err != nil {
		logger.Error("failed to validate get url", servicelog.Error(err))
		return err
	}
	logger = logger.With(servicelog.String("getURL", getURL))
	// Build the request for PUT
	var bo backoff.BackOff = eternalBackoff()
	if opts.maxRetries > 0 {
		bo = backoff.WithMaxRetries(bo, uint64(opts.maxRetries))
	}
	err = backoff.Retry(func() (returnErr error) {
		defer func() {
			returnErr = PermanentIfCancel(ctx, returnErr)
		}()
		var resp *http.Response
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
		if err != nil {
			logger.Error("failed to build request", servicelog.Error(err))
			return &backoff.PermanentError{Err: err}
		}
		resp, err = s.auth.Do(ctx, req, authChan)
		if resp != nil {
			defer exhaust(resp.Body)
		}
		if err != nil {
			logger.Error("failed to get data", servicelog.Error(err))
			return err
		}
		if resp.StatusCode < 200 || resp.StatusCode > 204 {
			err := bodyToError(resp)
			logger.Error("get request returned error", servicelog.Error(err))
			return err
		}
		if err := resource.ReadBody(resp.Body); err != nil {
			logger.Error("failed to process request data", servicelog.Error(err))
			return &backoff.PermanentError{Err: err}
		}
		return nil
	}, backoff.WithContext(bo, ctx))
	bo.Reset()
	return err
}
