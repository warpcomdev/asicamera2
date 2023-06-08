package watcher

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type stringError string

// Error implements error
func (err stringError) Error() string {
	return string(err)
}

const (
	// Error returned when the event channel for a folder is closed
	ChannelClosedError = stringError("channel closed")
	NotDirectoryError  = stringError("path must be a directory")
)

type FileHistory struct {
	lastUpdate    atomic.Time
	fileTypes     map[string]struct{}
	logger        *zap.Logger
	server        Server
	folder        string
	historyFolder string
	historyFile   string
	history       map[string]fileTask
	monitorFor    time.Duration
}

// New creates a new FileHistory object
func New(logger *zap.Logger, historyFolder string, server Server, folder string, fileTypes []string, monitorFor time.Duration) (*FileHistory, error) {
	// Generate unique history file name from folder name
	hash := fnv.New64a()
	hash.Write([]byte(folder))
	sum := hash.Sum64()
	filename := fmt.Sprintf("%s.%s", strconv.FormatUint(sum, 16), "csv")
	historyFile := filepath.Join(historyFolder, filename)
	// Create the file history
	f := &FileHistory{
		logger:        logger,
		server:        server,
		folder:        folder,
		historyFolder: historyFolder,
		historyFile:   historyFile,
		history:       nil,
		fileTypes:     make(map[string]struct{}),
		monitorFor:    monitorFor,
	}
	// Add the file types
	for _, fileType := range fileTypes {
		f.fileTypes[strings.ToLower(fileType)] = struct{}{}
	}
	return f, nil
}

// Return the time of the last file updated in the folder
func (f *FileHistory) LastUpdate() time.Time {
	return f.lastUpdate.Load()
}

// Watch the folder for changes
func (f *FileHistory) Watch(ctx context.Context) error {
	// Make sure the folder exists
	absPath, err := filepath.Abs(f.folder)
	if err != nil {
		f.logger.Error("failed to abs folder", zap.String("folder", absPath), zap.Error(err))
		return err
	}
	stat, err := os.Stat(absPath)
	if err != nil {
		f.logger.Error("failed to stat folder", zap.String("folder", absPath), zap.Error(err))
		return err
	}
	// Make sure it is a directory
	if !stat.IsDir() {
		f.logger.Error("path is not a directory", zap.String("folder", absPath))
		return NotDirectoryError
	}
	// Load the history file
	if err := f.loadHistory(); err != nil {
		f.logger.Error("failed to load history", zap.String("folder", absPath), zap.Error(err))
		return err
	}
	// Cleanup history tasks after we are done
	defer func() {
		for _, task := range f.history {
			if task.Events != nil {
				close(task.Events)
			}
		}
		f.history = nil
	}()
	// Create a notify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		f.logger.Error("failed to create watcher", zap.String("folder", absPath), zap.Error(err))
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
				f.logger.Error("watcher error", zap.String("folder", absPath), zap.Error(err))
				watcherErr = err
				return
			case <-failContext.Done():
				return
			}
		}
	}()
	// Merge real and synthetic events into a single channel
	syntheticEvents := make(chan fsnotify.Event, 16)
	defer close(syntheticEvents)
	events := make(chan fsnotify.Event, 16)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(events)
		f.merge(failContext, watcher.Events, syntheticEvents, events)
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
				break
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
		f.logger.Error("failed to watch folder", zap.String("folder", absPath), zap.Error(err))
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

// Merge info from two channel into one
func (f *FileHistory) merge(ctx context.Context, input1, input2, output chan fsnotify.Event) {
	// screen events by extension, only matched extensions are forwarded
	screen := func(event fsnotify.Event) {
		ext := strings.ToLower(filepath.Ext(event.Name))
		f.logger.Debug("screening event", zap.String("file", event.Name), zap.String("ext", ext))
		if _, ok := f.fileTypes[ext]; !ok {
			f.logger.Debug("Unrecognized extension", zap.String("file", event.Name))
		} else {
			output <- event
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-input1:
			// This will cause the events channel to close
			// and the dispatch goroutine to exit with a
			// ClosedChannelError
			if !ok {
				return
			}
			screen(event)
			break
		case event, ok := <-input2:
			if !ok {
				return
			}
			screen(event)
			break
		}
	}
}

// Load history file. The history file is a list of lines with date and path
func (f *FileHistory) loadHistory() error {
	// Make sure the history folder exists
	if _, err := os.Stat(f.historyFolder); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// Create the history folder
		if err := os.MkdirAll(f.historyFolder, 0755); err != nil {
			return err
		}
	}
	history := make(map[string]fileTask)
	// Read the file, if it exists
	file, err := os.Open(f.historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			f.history = history
			return nil
		}
		return err
	}
	defer file.Close()
	// If the file exists, it is kind of a raw csv
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			f.logger.Warn("invalid history line", zap.String("line", line))
			continue
		}
		timestamp, err := time.Parse(time.RFC3339, parts[0])
		if err != nil {
			f.logger.Warn("invalid date in history line", zap.String("line", line), zap.Error(err))
			continue
		}
		fname := parts[1]
		if _, err := os.Stat(fname); err != nil {
			f.logger.Warn("file from history no longer exists", zap.String("file", fname), zap.Error(err))
			continue
		}
		history[fname] = fileTask{
			Path:     fname,
			Uploaded: timestamp,
		}
	}
	f.history = history
	return nil
}

// Dispatch events until context is cancelled
func (f *FileHistory) dispatch(ctx context.Context, absPath string, events chan fsnotify.Event) error {
	tasks := make(chan fileTask, 16)
	defer close(tasks)
	remap := time.NewTicker(24 * time.Hour)
	defer remap.Stop()
	f.logger.Info("started dispatching events", zap.String("folder", absPath))
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		select {
		case <-ctx.Done():
			f.logger.Debug("event dispatch cancelled", zap.String("folder", absPath))
			return context.Canceled
		case <-remap.C:
			// Make sure the map does not grow infinite with stale entries
			// removed after a file is erased
			f.logger.Debug("remapping file history")
			newMap := make(map[string]fileTask)
			for _, task := range f.history {
				newMap[task.Path] = task
			}
			f.history = newMap
			break
		case event, ok := <-events:
			if !ok {
				f.logger.Debug("stopping folder watcher", zap.String("folder", absPath))
				return ChannelClosedError
			}
			f.logger.Debug("detected file event", zap.String("file", event.Name))
			fullName := filepath.Join(absPath, filepath.Base(event.Name))
			// If a file is removed, we must remove the entry in the log
			if event.Op&fsnotify.Remove == fsnotify.Remove {
				f.logger.Info("file removed", zap.String("file", fullName))
				task, taskExist := f.history[fullName]
				if taskExist {
					if task.Events != nil {
						close(task.Events)
						task.Events = nil
					}
					delete(f.history, fullName)
				}
			} else {
				// If a file is renamed, we must watch it until it is complete.
				// We can't delete it from the map, though, because we don't know
				// the prev name.
				mustUpdate := event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Rename)
				if mustUpdate {
					task, taskExist := f.history[fullName]
					f.logger.Debug("dispatch detected file", zap.String("file", fullName))
					// Make sure there is a task for this file
					if !taskExist {
						task = fileTask{
							Path: fullName,
						}
					}
					// Make sure the task is expecting events (ready for updates)
					if task.Events == nil {
						task.Events = make(chan fsnotify.Event, 1)
						wg.Add(1)
						go func() {
							defer wg.Done()
							f.logger.Info("started monitoring file", zap.String("file", task.Path))
							go task.upload(ctx, f.logger, f.server, task.Events, tasks, f.monitorFor)
						}()
					}
					// Update the task in the map, in case we created or modified it
					f.history[fullName] = task
					// Notify the task of a change in the file
					task.Events <- event
				}
			}
			break
		case task := <-tasks:
			// Get the original task object, in case we need
			// to close the events channel. We cannot close it
			// from the task object, because it is a copy,
			// and the original task might have been updated
			// in the loop.
			originalTask, taskExists := f.history[task.Path]
			if taskExists {
				if originalTask.Events != nil {
					f.logger.Info("stopping monitoring of file", zap.String("file", task.Path))
					close(originalTask.Events)
					originalTask.Events = nil
				}
			} else {
				originalTask = fileTask{
					Path: task.Path,
				}
			}
			if !task.Uploaded.IsZero() {
				originalTask.Uploaded = task.Uploaded
			}
			f.history[task.Path] = originalTask
			f.updateHistory()
			break
		}
	}
}

// scan the folder and send all updates to the channel
func (f *FileHistory) scan(ctx context.Context, absPath string, events chan fsnotify.Event) error {
	entries, err := os.ReadDir(absPath)
	if err != nil {
		f.logger.Error("failed to read folder", zap.String("folder", absPath), zap.Error(err))
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		f.logger.Info("scan detected file", zap.String("file", entry.Name()))
		event := fsnotify.Event{
			Name: entry.Name(),
			Op:   fsnotify.Create,
		}
		select {
		case <-ctx.Done():
			return context.Canceled
		case events <- event:
			break
		}
	}
	return nil
}

// Update the upload history
func (f *FileHistory) updateHistory() error {
	f.logger.Debug("updating log file", zap.String("file", f.historyFile))
	var lastUpdate time.Time
	history := make(map[string]string, len(f.history))
	for path, data := range f.history {
		// Only save files actually modified
		if !data.Uploaded.IsZero() {
			history[path] = data.Uploaded.Format(time.RFC3339)
			if data.Uploaded.After(lastUpdate) {
				lastUpdate = data.Uploaded
			}
		}
	}
	f.lastUpdate.Store(lastUpdate)
	logFolder := filepath.Dir(f.historyFile)
	file, err := ioutil.TempFile(logFolder, "logHistory")
	if err != nil {
		f.logger.Error("failed to create temporary log file", zap.String("folder", logFolder), zap.Error(err))
		return err
	}
	defer func() {
		if file != nil {
			file.Close()
			os.Remove(file.Name())
		}
	}()
	buf := bufio.NewWriter(file)
	for path, date := range history {
		buf.WriteString(date)
		buf.WriteString(",")
		buf.WriteString(path)
		buf.WriteString("\n")
	}
	buf.Flush()
	file.Close()
	if err := os.Rename(file.Name(), f.historyFile); err != nil {
		f.logger.Error("failed to rename temporary log file", zap.String("tmpFile", file.Name()), zap.String("file", f.historyFile))
		return err
	}
	file = nil // prevent deletion
	return nil
}
