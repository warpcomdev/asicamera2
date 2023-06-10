package watcher

import (
	"context"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// Server is the interface that must be implemented by the server
type Server func(ctx context.Context, path string) error

// fileTask is a file that needs to be uploaded
type fileTask struct {
	Path     string
	Uploaded time.Time
	Events   chan fsnotify.Event
}

func (t fileTask) upload(ctx context.Context, logger *zap.Logger, server Server, events chan fsnotify.Event, tasks chan<- fileTask, monitorFor time.Duration) {
	// Notify when we are done
	defer func() {
		tasks <- t
	}()
	// Notifications arrive WHILE THE FILE IS BEING UPDATED,
	// so we must wait a safe time before trying to upload the file.
	// Otherwise, we might upload an incomplete file,
	// or upload it more than once.
	// Since the exposition might be long, we will be waiting for a long time.
	// (up to 30 minutes per file)
	inactivity := time.NewTimer(monitorFor)
	defer inactivity.Stop()
	triggered := make(chan int, 1)
	defer close(triggered)
	// This queue will be used to notify of triggersDone triggers
	triggersDone := make(chan int, 1)
	triggersSent := 0
	go func() {
		defer close(triggersDone)
		for triggerNum := range triggered {
			// update the modified time of the file we uploaded.
			// triggers are run sequentially, so there is no race
			// condition here.
			t.Uploaded = t.triggered(ctx, logger, server)
			triggersDone <- triggerNum
		}
	}()
	// This loops watches for events until the file stops changing
	for {
		select {
		case triggerNumber, ok := <-triggersDone:
			// If an upload is completed, we must check the sequence number.
			// if it matches the last request triggered, then we are good to leave.
			// otherwise, we must wait for another completion
			logger.Debug("upload completed", zap.String("file", t.Path), zap.Int("trigger", triggerNumber))
			if !ok || triggerNumber >= triggersSent-1 {
				return
			}
			break
		case <-inactivity.C:
			// When the inactivity timer expires, trigger an upload
			logger.Debug("inactivity expired, triggering upload", zap.String("file", t.Path), zap.Duration("monitorFor", monitorFor))
			select {
			case triggered <- triggersSent:
				// Update the sequence number so we know which trigger
				// to wait for
				triggersSent += 1
				break
			default:
			}
			break
		case _, ok := <-events:
			// If the event channel is closed, the file has been removed
			// and we are no longer interested in uploading it. Quit.
			if !ok {
				logger.Debug("file removed, quitting", zap.String("file", t.Path))
				return
			}
			logger.Debug("reset of inactivity timer", zap.String("file", t.Path))
			// Otherwise, reset the inactivity timer
			if !inactivity.Stop() {
				<-inactivity.C
			}
			inactivity.Reset(monitorFor)
			break
		}
	}
}

func (t fileTask) triggered(ctx context.Context, logger *zap.Logger, server Server) time.Time {
	// The upload has been triggered!
	logger = logger.With(zap.String("file", t.Path), zap.Time("uploaded", t.Uploaded))
	info, err := os.Stat(t.Path)
	if err != nil {
		logger.Error("failed to stat file", zap.Error(err))
		return t.Uploaded
	}
	// BEWARE: modtime reports time in nanoseconds, but the history file
	// for some reason only saves with resolution of seconds. So we must round before
	// comparing, otherwise we always upload.
	modtime := info.ModTime().Round(time.Second)
	if !modtime.After(t.Uploaded) {
		// The file has not been modified since the last upload
		logger.Debug("file not modified")
		return t.Uploaded
	}
	logger.Debug("uploading file", zap.Time("modtime", modtime))
	// try to upload the file to the server
	if err := server(ctx, t.Path); err != nil {
		logger.Error("failed to upload file", zap.String("file", t.Path), zap.Error(err))
		return t.Uploaded
	}
	return info.ModTime()
}
