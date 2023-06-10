package main

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/warpcomdev/asicamera2/internal/driver/camera/backend"
	"github.com/warpcomdev/asicamera2/internal/driver/watcher"
	"go.uber.org/zap"
)

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
	// start folder watcher
	folderChan := make(chan string, 16)
	defer close(folderChan)
	wg.Add(1)
	go func() {
		defer wg.Done()
		server.WatchFolder(ctx, authChan, folderChan, time.Duration(config.ApiRefreshMinutes)*time.Minute)
	}()
	updateFunc := func(ctx context.Context, path string) error {
		ext := normalizeExtension(filepath.Ext(path))
		mimeType, ok := config.MimeTypes[ext]
		if !ok {
			logger.Error("failed to detect media type", zap.String("path", path))
			return nil
		}
		return server.Media(ctx, authChan, mimeType, path)
	}
	// Each tme the update folder ccanges, create a new watcher
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
		// Keep trying to watch folder until the name changes
		bo := backoff.NewExponentialBackOff()
		backoff.Retry(func() (returnError error) {
			defer func() {
				if returnError != nil {
					returnError = backend.PermanentIfCancel(ctx, returnError)
				}
			}()
			watch, err := watcher.New(logger, config.HistoryFolder, updateFunc, folderUpdate, config.FileTypes(), time.Duration(config.MonitorForMinutes)*time.Minute)
			if err != nil {
				return err
			}
			// Create a new folder watcher goroutine
			watcherCtx, watcherCancel := context.WithCancel(ctx)
			cancelPrevWatcher = watcherCancel
			wg.Add(1)
			go func() {
				defer wg.Done()
				bo := backoff.NewExponentialBackOff()
				backoff.Retry(func() (returnError error) {
					defer func() {
						if returnError != nil {
							returnError = backend.PermanentIfCancel(watcherCtx, returnError)
						}
					}()
					return watch.Watch(watcherCtx)
				}, bo)
			}()
			return nil
		}, bo)
	}
}
