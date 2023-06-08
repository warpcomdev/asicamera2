package dirsource

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// Source of frames
type Watcher struct {
	Updates chan string
	watcher *fsnotify.Watcher
	folders map[string]bool
}

// Matcher checks whether a folder name should be watched
type Matcher interface {
	MatchString(string) bool
}

func Start(logger *zap.Logger, root string, match Matcher) (*Watcher, error) {
	var (
		w   Watcher
		err error
	)
	w.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w.folders = make(map[string]bool)
	if err := w.rescan(logger, root, match); err != nil {
		w.watcher.Close()
		return nil, err
	}
	w.Updates = make(chan string, 1)
	go w.watch(logger, root, match)
	return &w, nil
}

func (w *Watcher) add(logger *zap.Logger, path string, match Matcher) error {
	// Read directorios and add to watcher
	subdirs, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	// Add the watcher to the path, if not already
	if _, found := w.folders[path]; !found {
		if err := w.watcher.Add(path); err != nil {
			return err
		}
	}
	w.folders[path] = true
	// Try to add all subdirs
	for _, subdir := range subdirs {
		subdirName := subdir.Name()
		if strings.HasPrefix(subdirName, ".") || !subdir.IsDir() {
			continue
		}
		if !match.MatchString(subdirName) {
			continue
		}
		fullPath := filepath.Join(path, subdir.Name())
		if err := w.add(logger, fullPath, match); err != nil {
			return err
		}
	}
	return nil
}

func (w *Watcher) Close() error {
	return w.watcher.Close()
}

func (w *Watcher) rescan(logger *zap.Logger, root string, match Matcher) error {
	// Set all folders to false and add again
	for path := range w.folders {
		w.folders[path] = false
	}
	if err := w.add(logger, root, match); err != nil {
		return err
	}
	// Locate all folders still false
	removeList := make([]string, 0, len(w.folders))
	for path, fresh := range w.folders {
		if !fresh {
			if err := w.watcher.Remove(path); err != nil {
				logger.Error("failed to remove watch", zap.String("path", path))
			} else {
				removeList = append(removeList, path)
			}
		}
	}
	// Remove false folders
	for _, path := range removeList {
		delete(w.folders, path)
	}
	// Replace folders with clean map
	cleanFolders := make(map[string]bool, len(w.folders))
	for path := range w.folders {
		cleanFolders[path] = true
	}
	w.folders = cleanFolders
	return nil
}

// Watch folder and subfolders
func (w *Watcher) watch(logger *zap.Logger, root string, match Matcher) {
	defer close(w.Updates)
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// Rescan every five minutes, in case we miss some directory
			if err := w.rescan(logger, root, match); err != nil {
				logger.Error("rescan failed", zap.Error(err))
			}
			break
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			logger.Info("notify event", zap.String("name", event.Name), zap.Int("op", int(event.Op)), zap.String("op_string", event.Op.String()))
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
				info, err := os.Stat(event.Name)
				if err != nil {
					logger.Error("failed to stat event", zap.String("name", event.Name), zap.Error(err))
				} else {
					if info.IsDir() {
						if _, found := w.folders[event.Name]; !found {
							if err := w.watcher.Add(event.Name); err != nil {
								logger.Error("failed to monitor folder", zap.String("name", event.Name), zap.Error(err))
							} else {
								logger.Info("monitoring new folder", zap.String("name", event.Name))
								w.folders[event.Name] = true
							}
						}
					} else {
						w.Updates <- event.Name
					}
				}
			}
			if event.Has(fsnotify.Remove) {
				if _, found := w.folders[event.Name]; found {
					if err := w.watcher.Remove(event.Name); err != nil {
						logger.Error("failed to remove folder", zap.String("name", event.Name), zap.Error(err))
					} else {
						logger.Info("stopped monitoring folder", zap.String("name", event.Name))
						delete(w.folders, event.Name)
					}
				}
			}
			break
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			logger.Error("notify watcher failed", zap.Error(err))
			break
		}
	}
}

func newestFile(logger *zap.Logger, root string, dirMatch Matcher, fileExt []string) (string, error) {
	// Locate newest File
	var (
		newestPath string
		newestTime time.Time
	)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logger.Error("failed to walk path", zap.String("path", path), zap.Error(err))
			return err
		}
		if d.IsDir() {
			return nil
		}
		parts := filepath.SplitList(path)
		numParts := len(parts)
		if numParts > 1 {
			if !dirMatch.MatchString(parts[numParts-2]) {
				return nil
			}
		}
		lower := strings.ToLower(parts[numParts-1])
		valid := false
		for _, ext := range fileExt {
			if strings.HasSuffix(lower, ext) {
				valid = true
				break
			}
		}
		if !valid {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		modTime := info.ModTime()
		if modTime.After(newestTime) {
			newestPath = path
			newestTime = modTime
		}
		return nil
	})
	return newestPath, err
}
