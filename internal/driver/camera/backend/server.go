package backend

import (
	"time"

	"go.uber.org/zap"
)

type Server struct {
	auth
	cameraID string
	queue    chan struct{}
}

type Config struct {
	ApiURL      string
	Username    string
	Password    string
	CameraID    string
	HTTPTimeout time.Duration
	Concurrency int
}

// Builds a new server
func New(logger *zap.Logger, client Client, config Config) *Server {
	concurrency := config.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	timeout := config.HTTPTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	server := &Server{
		auth: auth{
			logger:   logger,
			apiURL:   config.ApiURL,
			username: config.Username,
			password: config.Password,
			client:   client,
		},
		cameraID: config.CameraID,
		queue:    make(chan struct{}, concurrency),
	}
	for i := 0; i < concurrency; i++ {
		server.queue <- struct{}{}
	}
	return server
}
