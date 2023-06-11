package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/warpcomdev/asicamera2/internal/driver/camera/backend"
	"github.com/warpcomdev/asicamera2/internal/driver/watcher"
	"go.uber.org/zap"
)

type serverProxy struct {
	logger    *zap.Logger
	server    *backend.Server
	authChan  chan<- backend.AuthRequest
	wg        *sync.WaitGroup
	mimeTypes map[string]string
	cameraID  string
}

// Upload implements the watcher.Server interface
func (s serverProxy) Upload(ctx context.Context, path string) error {
	logger := s.logger
	ext := normalizeExtension(filepath.Ext(path))
	mimeType, ok := s.mimeTypes[ext]
	if !ok {
		logger.Error("failed to detect media type", zap.String("path", path))
		return nil
	}
	return s.server.Media(ctx, s.authChan, mimeType, path)
}

// Alert implements the watcher.Server interface
func (s serverProxy) Alert(ctx context.Context, id, severity, message string) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.server.Alert(ctx, s.authChan, id, severity, message)
	}()
}

func slowEternalBackoff() backoff.BackOff {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 5 * time.Second
	bo.Multiplier = 2
	bo.MaxInterval = 5 * time.Minute
	bo.MaxElapsedTime = 0
	return bo
}

func watchMedia(ctx context.Context, logger *zap.Logger, config Config, server *backend.Server) {
	authChan := make(chan backend.AuthRequest, 16)
	defer close(authChan)
	var wg sync.WaitGroup
	defer wg.Wait()
	// Start auth watcher
	wg.Add(1)
	go func() {
		defer wg.Done()
		server.WatchAuth(ctx, authChan)
	}()
	// Proxy to handle to watcher tasks
	proxy := &serverProxy{
		logger:    logger,
		server:    server,
		authChan:  authChan,
		wg:        &wg,
		mimeTypes: config.MimeTypes,
		cameraID:  config.CameraID,
	}
	// start USB monitor
	wg.Add(1)
	go func() {
		defer wg.Done()
		monitorUSB(ctx, logger, config, proxy)
	}()
	// start folder watcher
	folderChan := make(chan string, 16)
	defer close(folderChan)
	wg.Add(1)
	go func() {
		defer wg.Done()
		server.WatchFolder(ctx, authChan, folderChan, time.Duration(config.ApiRefreshMinutes)*time.Minute)
	}()
	// Each time the update folder changes, create a new watcher
	var cancelPrevWatcher func()
	defer func() {
		if cancelPrevWatcher != nil {
			cancelPrevWatcher()
		}
	}()
	for folderUpdate := range folderChan {
		if cancelPrevWatcher != nil {
			cancelPrevWatcher()
			cancelPrevWatcher = nil
		}
		logger = logger.With(zap.String("folder", folderUpdate))
		// Keep trying to watch until the folder name changes
		watch := watcher.New(logger, config.HistoryFolder, proxy, folderUpdate, config.FileTypes(), time.Duration(config.MonitorForMinutes)*time.Minute)
		watcherCtx, watcherCancel := context.WithCancel(ctx)
		cancelPrevWatcher = watcherCancel
		wg.Add(1)
		go func(folderUpdate string) {
			defer wg.Done()
			bo := slowEternalBackoff()
			alertID := fmt.Sprintf("watch-folder-%s", config.CameraID)
			backoff.Retry(func() (returnError error) {
				defer func() {
					if returnError != nil {
						logger.Error("folder watcher failed", zap.Error(returnError))
						proxy.Alert(ctx, alertID, "error", returnError.Error())
						returnError = backend.PermanentIfCancel(watcherCtx, returnError)
					}
				}()
				logger.Info("started watching folder")
				proxy.Alert(ctx, alertID, "info", "started watching folder "+folderUpdate)
				return watch.Watch(watcherCtx)
			}, backoff.WithContext(bo, watcherCtx))
		}(folderUpdate)
	}
}
