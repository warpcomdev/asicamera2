package watcher

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
)

// FileWatch watches changes in a Folder. Several FileWatches can share
// the same history file.
type FileWatch struct {
	FileHistory *FileHistory
	logger      servicelog.Logger
	fileTypes   map[string]struct{}
	server      Server
	folder      string
	monitorFor  time.Duration
}

// New creates a new FileWatch object
func New(logger servicelog.Logger, historyFolder string, server Server, folder string, fileTypes map[string]struct{}, monitorFor time.Duration, expiration time.Duration) *FileWatch {
	// Generate unique history file name from folder name
	hash := fnv.New64a()
	hash.Write([]byte(folder))
	sum := hash.Sum64()
	filename := fmt.Sprintf("%s.%s", strconv.FormatUint(sum, 16), "csv")
	historyFile := filepath.Join(historyFolder, filename)
	// Create the file history
	f := &FileWatch{
		FileHistory: NewHistory(logger, historyFolder, historyFile, expiration),
		logger:      logger,
		server:      server,
		folder:      folder,
		fileTypes:   fileTypes,
		monitorFor:  monitorFor,
	}
	return f
}

// Watch the folder for changes
func (f *FileWatch) Watch(ctx context.Context) error {
	// Make sure the folder exists
	absPath, err := filepath.Abs(f.folder)
	logger := f.logger.With(servicelog.String("folder", absPath))
	if err != nil {
		logger.Error("failed to abs folder", servicelog.Error(err))
		return err
	}
	stat, err := os.Stat(absPath)
	if err != nil {
		logger.Error("failed to stat folder", servicelog.Error(err))
		return err
	}
	// Make sure it is a directory
	if !stat.IsDir() {
		logger.Error("path is not a directory")
		return NotDirectoryError
	}
	// Load the history file
	if err := f.FileHistory.Load(); err != nil {
		logger.Error("failed to load history", servicelog.Error(err))
		return err
	}
	// Cleanup history tasks after we are done
	defer func() {
		f.FileHistory.Cleanup()
	}()
	// Create a notify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error("failed to create watcher", servicelog.Error(err))
		return err
	}
	defer watcher.Close()
	// Start listening for events.
	failContext, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		wg          sync.WaitGroup
		dispatchErr error
		watcherErr  error
	)
	// Watch the error channel
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel() // Cancel the context if the watcher errors
		for {
			select {
			case err, ok := <-watcher.Errors:
				if !ok {
					watcherErr = ChannelClosedError
					return
				}
				logger.Error("watcher error", servicelog.Error(err))
				watcherErr = err
				return
			case <-failContext.Done():
				return
			}
		}
	}()
	// Merge events both from file and from synthetic events
	syntheticEvents := make(chan fsnotify.Event, 16)
	defer close(syntheticEvents)
	events := make(chan fsnotify.Event, 16)
	// This screener filters events by extension, only matched extensions are forwarded
	screen := func(event fsnotify.Event) {
		logger := f.logger.With(servicelog.String("file", event.Name))
		ext := strings.ToLower(filepath.Ext(event.Name))
		logger.Debug("screening event", servicelog.String("ext", ext))
		// Check if it is a new directory. We don't neeed to watch for renames
		// because the watcher will do that automatically.
		if event.Has(fsnotify.Create) {
			stat, err := os.Stat(event.Name)
			if err != nil {
				logger.Error("failed to stat file", servicelog.Error(err))
				return
			}
			if stat.IsDir() {
				if strings.HasSuffix(event.Name, ".") {
					logger.Debug("skipping directory")
					return
				}
				err := watcher.Add(event.Name)
				if err != nil {
					logger.Error("failed to add directory watcher", servicelog.Error(err))
					// it will be retried on next scan
				}
				logger.Debug("watching new directory")
				return
			}
		}
		// Otherwise, check if it is an interesting file
		if ext != "" {
			if _, ok := f.fileTypes[ext]; !ok {
				logger.Debug("Unrecognized extension", servicelog.String("ext", ext))
			} else {
				events <- event
			}
		}
	}
	// Merge real and synthetic events into a single channel
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(events)
		f.merge(failContext, watcher.Events, syntheticEvents, screen)
	}()
	// Generate synthetic events periodically
	wg.Add(1)
	go func() {
		defer wg.Done()
		timer := time.NewTimer(0)
		for {
			select {
			case <-timer.C:
				f.scan(failContext, absPath, syntheticEvents)
				timer.Reset(2 * time.Hour)
			case <-failContext.Done():
				return
			}
		}
	}()
	// Dispatch file events
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		dispatchErr = f.dispatch(failContext, absPath, events)
	}()
	// Add a path to watch
	notifyErr := watcher.Add(absPath)
	if notifyErr != nil {
		logger.Error("failed to watch folder", servicelog.Error(err))
		cancel()
	}
	wg.Wait()
	// If all errors are "context cancelled", we are good
	if (notifyErr == nil || errors.Is(notifyErr, context.Canceled)) &&
		(dispatchErr == nil || errors.Is(dispatchErr, context.Canceled)) &&
		(watcherErr == nil || errors.Is(watcherErr, context.Canceled)) {
		return context.Canceled
	}
	return errors.Join(notifyErr, dispatchErr, watcherErr)
}

// Merge info from two channels into a single forwarder
func (f *FileWatch) merge(ctx context.Context, input1, input2 chan fsnotify.Event, forward func(fsnotify.Event)) {
	// screen events by extension, only matched extensions are forwarded
	logger := f.logger
	for {
		select {
		case <-ctx.Done():
			logger.Error("merge context cancelled")
			return
		case event, ok := <-input1:
			// This will cause the events channel to close
			// and the dispatch goroutine to exit with a
			// ClosedChannelError
			if !ok {
				logger.Error("merge first channel closed")
				return
			}
			forward(event)
		case event, ok := <-input2:
			if !ok {
				logger.Error("merge second channel closed")
				return
			}
			forward(event)
		}
	}
}

// Dispatch events until context is cancelled
func (f *FileWatch) dispatch(ctx context.Context, absPath string, events chan fsnotify.Event) error {
	var wg sync.WaitGroup
	tasks := make(chan fileTask, 16)
	defer func() {
		// Wait until all goroutines that might
		// write to tasks are done
		wg.Wait()
		// close and exhaust pending tasks
		close(tasks)
		for range tasks {
		}
	}()
	remap := time.NewTicker(24 * time.Hour)
	defer remap.Stop()
	f.logger.Info("started dispatching events")
	// Make sure we cancel all tasks if we exit for something besides main context cancellation
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()
	for {
		select {
		case <-ctx.Done():
			f.logger.Debug("event dispatch cancelled")
			return context.Canceled
		// Put task dispatching before event dispatching. In case there is a
		// burst of tasks, we want to process dispatchs as they arrive,
		// giving priority over new tasks
		case task, ok := <-tasks:
			if !ok {
				f.logger.Debug("stopping task watcher")
				return ChannelClosedError
			}
			f.FileHistory.CompleteTask(task)
			f.FileHistory.Save()
		case <-remap.C:
			// Make sure the map does not grow infinite with stale entries
			// removed after a file is erased
			f.logger.Debug("remapping file history")
			f.FileHistory.Remap()
		case event, ok := <-events:
			if !ok {
				f.logger.Debug("stopping folder watcher")
				return ChannelClosedError
			}
			fullName := filepath.Join(event.Name)
			logger := f.logger.With(servicelog.String("file", fullName))
			logger.Debug("detected file event")
			// If a file is removed, we must remove the entry in the log
			if event.Op&fsnotify.Remove == fsnotify.Remove {
				logger.Info("file removed")
				f.FileHistory.RemoveTask(fullName)
			} else {
				// If a file is renamed, we must watch it until it is complete.
				// We can't delete it from the map, though, because we don't know
				// the prev name.
				mustUpdate := event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Rename)
				if mustUpdate {
					logger.Debug("dispatch detected file")
					task, newChannel := f.FileHistory.CreateTask(fullName)
					// send the information on the channel before creating a goroutine,
					// to avoid having the inactivity timer trigger before there is actually
					// any change in the file
					select {
					case task.Events <- event:
					default:
						logger.Debug("failed dispatch to busy task")
					}
					// If the channel is new, start a new uploader routine
					if newChannel {
						wg.Add(1)
						go func() {
							defer wg.Done()
							logger.Info("started monitoring file")
							task.upload(cancelCtx, f.logger, f.server, tasks, f.monitorFor)
						}()
					}
				}
			}
		}
	}
}

// scan the folder and send all updates to the channel
func (f *FileWatch) scan(ctx context.Context, absPath string, events chan fsnotify.Event) error {
	entries, err := os.ReadDir(absPath)
	if err != nil {
		f.logger.Error("failed to read folder", servicelog.String("folder", absPath), servicelog.Error(err))
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			f.logger.Info("scan detected folder", servicelog.String("name", entry.Name()))
			// Recursively scan folder
			subPath := filepath.Join(absPath, entry.Name())
			if err := f.scan(ctx, subPath, events); err != nil {
				f.logger.Error("failed to scan subpath", servicelog.String("subpath", subPath))
			}
		} else {
			f.logger.Info("scan detected file", servicelog.String("name", entry.Name()))
		}
		event := fsnotify.Event{
			Name: filepath.Join(absPath, entry.Name()),
			Op:   fsnotify.Create,
		}
		select {
		case <-ctx.Done():
			f.logger.Error("scan context cancelled", servicelog.String("absPath", absPath))
			return context.Canceled
		case events <- event:
		}
	}
	return nil
}
