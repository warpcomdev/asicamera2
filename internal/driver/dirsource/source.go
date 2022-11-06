package dirsource

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/warpcomdev/asicamera2/internal/driver/jpeg"
	"go.uber.org/zap"
)

type frame struct {
	src      *jpeg.Image
	img      *jpeg.Image
	features jpeg.JpegFeatures
}

// Buffer implements jpeg.SrcFrame
func (f frame) Buffer() *jpeg.Image {
	return f.img
}

// Compress implements jpeg.SrcFrame
func (f frame) Compress(compressor jpeg.Compressor, target *jpeg.Image) (zero jpeg.JpegFeatures, err error) {
	if err := target.Copy(f.src); err != nil {
		return zero, err
	}
	return f.features, nil
}

// Source of frames
type Source struct {
	// Path to image folder
	root string
	// Decompression resources
	decompressor jpeg.Decompressor
	image        jpeg.Image
	// Latest decompressed image
	mutex       sync.Mutex
	currentPath string
	currentDate time.Time
	newestPath  string
	newestDate  time.Time
	newestData  frame
	// frame rate
	watcher *Watcher
	rate    *time.Ticker
}

// Name implements jpeg.Source
func (s *Source) Name() string {
	return s.root
}

func (s *Source) Next(ctx context.Context, img *jpeg.Image) (jpeg.SrcFrame, error) {
	for {
		select {
		case <-s.rate.C:
			s.mutex.Lock()
			currentPath, newestPath := s.currentPath, s.newestPath
			currentDate, newestDate := s.currentDate, s.newestDate
			s.mutex.Unlock()
			if currentPath != newestPath || newestDate.After(currentDate) {
				dirname, filename := filepath.Split(newestPath)
				fsys := os.DirFS(dirname)
				features, err := jpeg.Decompressor.ReadFile(s.decompressor, fsys, filename, &s.image)
				if err != nil {
					return nil, fmt.Errorf("failed to read file %s: %w", s.newestPath, err)
				}
				s.newestData = frame{
					src:      &s.image,
					img:      img,
					features: features,
				}
				s.mutex.Lock()
				s.currentPath = s.newestPath
				s.currentDate = s.newestDate
				s.mutex.Unlock()
			}
			if s.image.Size() > 0 {
				return s.newestData, nil
			}
		case <-ctx.Done():
			return nil, errors.New("context cancelled")
		}
	}
}

func (rs *Source) Start(logger *zap.Logger) error {
	var err error
	rs.watcher, err = Start(logger, rs.root)
	if err != nil {
		return err
	}
	// Start listener gopher for updates
	go func(watcher *Watcher) {
		for path := range watcher.Updates {
			info, err := os.Stat(path)
			if err != nil {
				logger.Error("failed to stat file", zap.String("path", path), zap.Error(err))
			} else {
				func() {
					rs.mutex.Lock()
					defer rs.mutex.Unlock()
					rs.newestPath = path
					rs.newestDate = info.ModTime()
				}()
			}
		}
	}(rs.watcher)
	// seed with newest file
	newest, err := newestFile(logger, rs.root)
	if err != nil {
		logger.Error("failed to get newest file", zap.Error(err))
	} else {
		rs.watcher.Updates <- newest
	}
	// Set frame rate and decompressor
	rs.rate = time.NewTicker(time.Second)
	rs.decompressor = jpeg.NewDecompressor()
	return nil
}

func (rs *Source) Stop() {
	// Close watcher
	if err := rs.watcher.Close(); err != nil {
		// Leaving file watches open might be a huge performance problem,
		// it's better to abort and let the service restart
		panic(err)
	}
	// Exhaust updates
	for range rs.watcher.Updates {
	}
	// Free frame rate and decompressor
	rs.rate.Stop()
	rs.decompressor.Free()
	rs.image.Free()
}

func New(logger *zap.Logger, root string) (*Source, error) {
	return &Source{root: root}, nil
}
