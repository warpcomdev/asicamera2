package backend

import (
	"context"

	"go.uber.org/zap"
)

type Server struct {
	logger *zap.Logger
}

// Uploads a video to the server
func (s *Server) Upload(ctx context.Context, path string) error {
	s.logger.Info("uploading file", zap.String("path", path))
	return nil
}

// Builds a new server
func New(logger *zap.Logger) (*Server, error) {
	return &Server{
		logger: logger,
	}, nil
}

// Returns the folder to watch
func (s *Server) Folder(ctx context.Context) (string, error) {
	return "C:\\XboxGames\\test", nil
}
