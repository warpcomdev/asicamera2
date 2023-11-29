package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/warpcomdev/asicamera2/internal/driver/backend"
	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
	"github.com/warpcomdev/asicamera2/internal/driver/watcher"
)

type serverProxy struct {
	logger          servicelog.Logger
	server          *backend.Server
	authChan        chan<- backend.AuthRequest
	wg              *sync.WaitGroup
	mimeTypes       map[string]string
	cameraID        string
	cameraKeepalive chan struct{}
}

// Upload implements the watcher.Server interface
func (s serverProxy) Upload(ctx context.Context, path string) error {
	logger := s.logger
	ext := normalizeExtension(filepath.Ext(path))
	mimeType, ok := s.mimeTypes[ext]
	if !ok {
		logger.Error("failed to detect media type", servicelog.String("path", path))
		return errors.New("failed to detect media type")
	}
	// Notify the keepalive channel there is a new update attempt
	select {
	case s.cameraKeepalive <- struct{}{}:
	default:
	}
	return s.server.Media(ctx, s.authChan, mimeType, path)
}

// SendAlert implements the watcher.Server interface
func (s serverProxy) SendAlert(ctx context.Context, id, name, severity, message string) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.server.SendAlert(ctx, s.authChan, id, name, severity, message)
	}()
}

// ClearAlert implements the watcher.Server interface
func (s serverProxy) ClearAlert(ctx context.Context, id string) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.server.ClearAlert(ctx, s.authChan, id)
	}()
}

// CameraID implements the watcher.Server interface
func (s serverProxy) CameraID() string {
	return s.cameraID
}

func slowEternalBackoff() backoff.BackOff {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 5 * time.Second
	bo.Multiplier = 2
	bo.MaxInterval = 5 * time.Minute
	bo.MaxElapsedTime = 0
	return bo
}

func watchMedia(ctx context.Context, logger servicelog.Logger, config Config, server *backend.Server) {
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
		logger:          logger,
		server:          server,
		authChan:        authChan,
		wg:              &wg,
		mimeTypes:       config.MimeTypes,
		cameraID:        config.CameraID,
		cameraKeepalive: make(chan struct{}, 1),
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
	// start update watcher. Send an alert if there are no updates to
	// the folder in 24 hours
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTimer(24 * time.Hour)
		select {
		case <-proxy.cameraKeepalive:
			// Stop and drain the timer
			if t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			// expect up to 24 more hours
			t.Reset(24 * time.Hour)
		case <-t.C:
			// 24 hours without updates
			alertName := "camera_not_recording"
			alertID := fmt.Sprintf("%s_%s", alertName, proxy.CameraID())
			proxy.SendAlert(ctx, alertID, alertName, "warning", "No new recordings detected in 24 hours")
		case <-ctx.Done():
			return
		}
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
		logger = logger.With(servicelog.String("folder", folderUpdate))
		// Keep trying to watch until the folder name changes
		watch := watcher.New(logger,
			config.HistoryFolder,
			proxy,
			folderUpdate,
			config.FileTypes(),
			time.Duration(config.MonitorForMinutes)*time.Minute,
			time.Duration(config.ExpireAfterDays)*time.Hour*24,
			config.DenyList,
		)
		watcherCtx, watcherCancel := context.WithCancel(ctx)
		cancelPrevWatcher = watcherCancel
		wg.Add(1)
		// Do the folder watching in a separate goroutine, because
		// the process runs for as long as the context is not interrupted,
		// but we still must react if some new folder name arrives.
		go func(folderUpdate string) {
			defer wg.Done()
			alertName := "watch_folder"
			alertTime := time.Now()
			alertID := fmt.Sprintf("%s_%s_%s", config.CameraID, alertName, alertTime.Format(time.RFC3339))
			alertTriggered := false
			bo := slowEternalBackoff()
			backoff.Retry(func() (returnError error) {
				defer func() {
					if returnError != nil {
						logger.Error("folder watcher failed", servicelog.Error(returnError))
						returnError = backend.PermanentIfCancel(watcherCtx, returnError)
					}
				}()
				logger.Info("started watching folder")
				// We will use an aux goroutine to detect if the watcher has been running long enough or not,
				// and trigger an alert depending on the case
				var (
					watcherWG sync.WaitGroup
					resetBO   bool
					stop      = make(chan struct{}, 1)
				)
				defer close(stop)
				watcherWG.Add(1)
				// Send or clear an alarm, depending on how long does the watcher take
				go func() {
					defer watcherWG.Done()
					select {
					case <-watcherCtx.Done():
						return
					case <-time.After(30 * time.Second):
						// the watcher has been running for 30 seconds,
						// I think it's ok to clear the alert
						if alertTriggered {
							proxy.ClearAlert(ctx, alertID)
						}
						// Update alertID for next time
						alertTime = time.Now()
						alertID = fmt.Sprintf("%s_%s_%s", config.CameraID, alertName, alertTime.Format(time.RFC3339))
						alertTriggered = false
						// And reset backoff
						resetBO = true
					case <-stop:
						// stopped before the timer, looks like the watcher didn't work...
						if !alertTriggered {
							proxy.SendAlert(ctx, alertID, alertName, "error", returnError.Error())
							alertTriggered = true
						}
					}
				}()
				err := watch.Watch(watcherCtx)
				watcherWG.Wait()
				// If the alert goroutine says we must reset the backoff, do it
				// in this same goroutine (bo object is not reentrant)
				if resetBO {
					bo.Reset()
				}
				return err
			}, backoff.WithContext(bo, watcherCtx))
		}(folderUpdate)
	}
}
