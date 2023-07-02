package watcher

import (
	"bufio"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
	"go.uber.org/atomic"
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
	logger        servicelog.Logger
	historyFolder string
	historyFile   string
	history       map[string]fileTask
	expiration    time.Duration
}

// New creates a new FileHistory object
func NewHistory(logger servicelog.Logger, historyFolder, historyFile string, expiration time.Duration) *FileHistory {
	// Create the file history
	f := &FileHistory{
		logger:        logger,
		historyFolder: historyFolder,
		historyFile:   historyFile,
		history:       make(map[string]fileTask),
		expiration:    expiration,
	}
	return f
}

// Return the time of the last file updated in the folder
func (f *FileHistory) LastUpdate() time.Time {
	return f.lastUpdate.Load()
}

// Remap rebuilds the history map, to clean any sparse entries
// after many adds and removals
func (f *FileHistory) Remap() {
	newMap := make(map[string]fileTask)
	for _, task := range f.history {
		// Remove files that have been uploaded for long enough
		keep := true
		if !task.Uploaded.IsZero() && f.expiration > 0 && time.Since(task.Uploaded) > f.expiration {
			logger := f.logger.With(servicelog.String("path", task.Path))
			err := os.Remove(task.Path)
			if err == nil {
				logger.Info("removed expired file from history")
				keep = false
			} else {
				// If there was an error, check if it was file not exist
				// or path is a directory
				if os.IsNotExist(err) {
					logger.Info("cleaned missing file from history")
					keep = false
				} else {
					stat, statErr := os.Stat(task.Path)
					if statErr == nil {
						if stat.IsDir() {
							logger.Info("cleaned directory from history")
							keep = false
						}
					} else {
						if os.IsNotExist(statErr) {
							logger.Info("cleaned missing directory from history")
							keep = false
						} else {
							logger.Error("could not remove or stat file", servicelog.Error(err))
							err = errors.Join(err, statErr)
						}
					}
				}
			}
			if keep {
				logger.Error("could not determine how to clean up expired file", servicelog.Error(err))
			}
		}
		if keep {
			newMap[task.Path] = task
		}
	}
	f.history = newMap
}

// Load history file. The history file is a list of lines with date and path
func (f *FileHistory) Load() error {
	logger := f.logger.With(servicelog.String("historyFile", f.historyFile))
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
			logger.Warn("invalid history line", servicelog.String("line", line))
			continue
		}
		timestamp, err := time.Parse(time.RFC3339, parts[0])
		if err != nil {
			logger.Warn("invalid date in history line", servicelog.String("line", line), servicelog.Error(err))
			continue
		}
		fname := parts[1]
		if _, err := os.Stat(fname); err != nil {
			logger.Warn("file from history no longer exists", servicelog.String("file", fname), servicelog.Error(err))
			continue
		}
		logger.Debug("loaded history line", servicelog.String("file", fname), servicelog.Time("date", timestamp))
		history[fname] = fileTask{
			Path:     fname,
			Uploaded: timestamp,
		}
	}
	f.history = history
	return nil
}

// Save the upload history
func (f *FileHistory) Save() error {
	f.logger.Debug("updating log file", servicelog.String("historyFile", f.historyFile))
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
		f.logger.Error("failed to create temporary log file", servicelog.String("folder", logFolder), servicelog.Error(err))
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
		f.logger.Error("failed to rename temporary log file", servicelog.String("tmpFile", file.Name()), servicelog.String("file", f.historyFile))
		return err
	}
	file = nil // prevent deletion
	return nil
}

// Get or create a task. The createFunc is called if either the task
// or the event channel are created.
func (f *FileHistory) CreateTask(fullName string, onCreate func(newTask fileTask)) fileTask {
	task, taskExist := f.history[fullName]
	// Make sure there is a task for this file
	if !taskExist {
		task = fileTask{
			Path: fullName,
		}
	}
	// Make sure the task is expecting events (ready for updates)
	if task.Events == nil {
		task.Events = make(chan fsnotify.Event, 1)
		onCreate(task)
	}
	// Update the task in the map, in case we created or modified it
	f.history[fullName] = task
	return task
}

// Cleanup must be called on termination. It closes all event channels
func (f *FileHistory) Cleanup() {
	for _, task := range f.history {
		if task.Events != nil {
			close(task.Events)
		}
	}
	f.history = nil
}

// RemoveTask removes the task from the history and closes the channel
func (f *FileHistory) RemoveTask(fullName string) {
	task, taskExist := f.history[fullName]
	if taskExist {
		if task.Events != nil {
			close(task.Events)
			task.Events = nil
		}
		delete(f.history, fullName)
	}
}

// CompleteTask marks the task as completed and closes the event channel.
func (f *FileHistory) CompleteTask(task fileTask) {
	originalTask, taskExists := f.history[task.Path]
	if taskExists {
		if originalTask.Events != nil {
			f.logger.Info("stopping monitoring of file", servicelog.String("file", task.Path))
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
	f.history[originalTask.Path] = originalTask
}
