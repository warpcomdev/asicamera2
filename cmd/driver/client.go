package main

import (
	"bytes"
	"io"
	"net/http"

	"github.com/warpcomdev/asicamera2/internal/driver/backend"
	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
)

type debugClient struct {
	logger servicelog.Logger
	client backend.Client
}

// Reader that keeps a buffer with the first 4kb of the request
type peekReader struct {
	reader io.ReadCloser
	buffer bytes.Buffer
}

// Read implements ReadCloser
func (r *peekReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		if r.buffer.Len() < 1024 {
			remaining := 1024 - r.buffer.Len()
			if remaining > n {
				remaining = n
			}
			if remaining > 0 {
				r.buffer.Write(p[:remaining])
			}
		}
	}
	return n, err
}

// Close implements ReadCloser
func (r *peekReader) Close() error {
	return r.reader.Close()
}

// Do implements Client
func (c debugClient) Do(req *http.Request) (*http.Response, error) {
	logger := c.logger.With(servicelog.String("method", req.Method), servicelog.String("url", req.URL.String()), servicelog.Any("headers", req.Header))
	/*var pr *peekReader
	if req.Body != nil {
		pr = &peekReader{
			reader: req.Body,
		}
		req.Body = pr
	}*/
	resp, err := c.client.Do(req)
	/*if pr != nil {
		logger = logger.With(servicelog.ByteString("body", pr.buffer.Bytes()))
	}*/
	logger.Debug("HTTP request")
	return resp, err
}
