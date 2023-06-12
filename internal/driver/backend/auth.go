package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/cenkalti/backoff"
	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
)

type serverError string

func (e serverError) Error() string {
	return string(e)
}

const (
	EmptyAuthResponseError   = serverError("empty auth response")
	EmptyTokenResponseError  = serverError("empty token response")
	EmptyFolderResponseError = serverError("empty folder response")
	AuthFailedError          = serverError("auth failed")
	AuthTokenExpiredError    = serverError("auth token expired")
	PostFailedError          = serverError("POST failed and there is no PUT")
	UnknownMediaTypeError    = serverError("unknown media type")
)

// Minimal surface of the http.Client we use
type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

type auth struct {
	logger   servicelog.Logger
	apiURL   string
	username string
	password string
	client   Client
}

type httpAuthRequest struct {
	ID       string `json:"id"`
	Password string `json:"password"`
}

type httpAuthReply struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	Token string `json:"token"`
}

// Turn context.Cancel error into permanent error (no retries)
func PermanentIfCancel(ctx context.Context, err error) error {
	if err == nil {
		return err
	}
	var permanent *backoff.PermanentError
	if errors.As(err, &permanent) {
		return err
	}
	if errors.Is(err, context.Canceled) {
		return &backoff.PermanentError{Err: err}
	}
	// If the context is cancelled, make sure the error is permanent
	select {
	case <-ctx.Done():
		return &backoff.PermanentError{Err: err}
	default:
		break
	}
	return err
}

func exhaust(body io.ReadCloser) {
	if body != nil {
		io.Copy(ioutil.Discard, body)
		body.Close()
	}
}

// httpAuth authenticates with the server and returns the httpAuth ID and token
// BEWARE: backoff is not thread-safe, do not share amongst goroutines
func (a auth) httpAuth(ctx context.Context, bo backoff.BackOff) (string, string, error) {
	logger := a.logger.With(servicelog.String("url", a.apiURL), servicelog.String("username", a.username))
	var (
		authID    string
		authToken string
		authErr   error
	)
	// Encode the auth body
	credentials := httpAuthRequest{
		ID:       a.username,
		Password: a.password,
	}
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	err := encoder.Encode(credentials)
	if err != nil {
		logger.Error("failed to encode credentials", servicelog.Error(err))
		return "", "", err
	}
	body := buffer.Bytes()
	// Parse the auth URL
	authURL, err := validateURL(a.apiURL + "/api/login")
	if err != nil {
		logger.Error("failed to parse auth url", servicelog.String("url", a.apiURL), servicelog.Error(err))
		return "", "", err
	}
	logger = logger.With(servicelog.String("authUrl", authURL))
	// Keep retrying until we succeed
	authErr = backoff.Retry(func() (returnErr error) {
		defer func() {
			returnErr = PermanentIfCancel(ctx, returnErr)
		}()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, bytes.NewReader(body))
		if err != nil {
			logger.Error("failed to create request", servicelog.Error(err))
			return &backoff.PermanentError{Err: err}
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := a.client.Do(req)
		if resp != nil {
			defer exhaust(resp.Body)
		}
		if err != nil {
			logger.Error("failed to authenticate", servicelog.Error(err))
			return err
		}
		if resp.Body == nil {
			logger.Error("empty auth response", servicelog.Error(err))
			return EmptyAuthResponseError
		}
		if resp.StatusCode != http.StatusOK {
			err = bodyToError(resp)
			logger.Error("authentication rejected", servicelog.Error(err))
			return err
		}
		var reply httpAuthReply
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(&reply); err != nil {
			logger.Error("failed to decode auth reply", servicelog.Error(err))
			return err
		}
		if reply.Token == "" {
			logger.Error("empty token response", servicelog.Error(err))
			return EmptyTokenResponseError
		}
		authID = reply.ID
		authToken = reply.Token
		return nil
	}, backoff.WithContext(bo, ctx))
	// reset the backoff after we retried
	bo.Reset()
	return authID, authToken, authErr
}

type AuthReply struct {
	ID     string
	Token  string
	Cached bool
	Err    error
}

type AuthRequest struct {
	Reply chan AuthReply
	fresh bool // whether to request fresh credentials
}

// Attend to authentication queries in the channel
func (a auth) WatchAuth(ctx context.Context, queries <-chan AuthRequest) {
	logger := a.logger.With(servicelog.String("apiUrl", a.apiURL), servicelog.String("username", a.username))
	bo := eternalBackoff()
	lastReply := AuthReply{}
	for {
		select {
		case <-ctx.Done():
			logger.Info("context cancelled")
			return
		case query, ok := <-queries:
			if !ok {
				logger.Info("auth queries channel closed")
				return
			}
			if query.fresh || lastReply.Token == "" {
				id, token, err := a.httpAuth(ctx, bo)
				lastReply = AuthReply{
					ID:     id,
					Token:  token,
					Cached: false,
					Err:    err,
				}
				if err != nil {
					logger.Error("failed to authenticate", servicelog.Error(err))
					lastReply.Token = ""
				}
			}
			query.Reply <- lastReply
			// Next one that asks for a cached creds, willl know it is cached
			lastReply.Cached = true
			close(query.Reply)
		}
	}
}

// Get an authentication response from the channel
func (a auth) getAuth(ctx context.Context, fresh bool, queries chan<- AuthRequest) (zero AuthReply, err error) {
	logger := a.logger
	query := AuthRequest{
		fresh: fresh,
		Reply: make(chan AuthReply, 1),
	}
	select {
	case <-ctx.Done():
		// The query has not been sent to the attender yet, close it.
		close(query.Reply)
		logger.Error("context cancelled")
		return zero, ctx.Err()
	case queries <- query:
		select {
		case <-ctx.Done():
			// The attender will close the channel
			logger.Error("context cancelled")
			return zero, ctx.Err()
		case reply, ok := <-query.Reply:
			if !ok {
				logger.Error("auth queries channel closed")
				return reply, ctx.Err()
			}
			return reply, reply.Err
		}
	}
}

// Do a request with authentication, retry if response is 401 Unauthorized or 403 Forbidden
func (a *auth) Do(ctx context.Context, req *http.Request, auth chan<- AuthRequest) (*http.Response, error) {
	reply, err := a.getAuth(ctx, false, auth)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+reply.Token)
	resp, err := a.client.Do(req)
	if err != nil {
		if resp != nil {
			exhaust(resp.Body)
		}
		return nil, err
	}
	if (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) && reply.Cached {
		// retry with new token
		reply, err = a.getAuth(ctx, true, auth)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+reply.Token)
		return a.client.Do(req)
	}
	return resp, nil
}
