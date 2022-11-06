package dirsource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	images       [2]jpeg.Image
	currentImage int

	// Latest decompressed image
	mutex      sync.Mutex
	newestData frame
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
			frame := func() jpeg.SrcFrame {
				s.mutex.Lock()
				defer s.mutex.Unlock()
				if s.images[s.currentImage].Size() <= 0 {
					return nil
				}
				return frame{
					src:      &s.images[s.currentImage],
					img:      img,
					features: s.newestData.features,
				}
			}()
			if frame != nil {
				return frame, nil
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
		latestFile := ""
		for path := range watcher.Updates {
			info, err := os.Stat(path)
			if err != nil {
				logger.Error("failed to stat file", zap.String("path", path), zap.Error(err))
				continue
			}
			if info.IsDir() {
				continue
			}
			lower := strings.ToLower(path)
			if !strings.HasSuffix(lower, ".jpg") && !strings.HasSuffix(lower, ".jpeg") {
				continue
			}
			// Do not send the same file several times in a row
			if path == latestFile {
				continue
			}
			// Introduce a bit of dalay before reading the file,
			// because we don't want to read before the software has finished writing
			go func(path string) {
				<-time.After(5 * time.Second)
				if err := rs.readImage(path); err != nil {
					logger.Error("failed to refresh image", zap.String("path", path), zap.Error(err))
				}
			}(path)
			latestFile = path
		}
	}(rs.watcher)
	// seed with newest file
	newest, err := newestFile(logger, rs.root, []string{".jpg", ".jpeg"})
	if err != nil {
		logger.Error("failed to get newest file", zap.Error(err))
	} else {
		if err := rs.readImage(newest); err != nil {
			logger.Error("failed to read file", zap.String("path", newest), zap.Error(err))
		}
	}
	// Set frame rate and decompressor
	rs.rate = time.NewTicker(time.Second)
	rs.decompressor = jpeg.NewDecompressor()
	return nil
}

func (rs *Source) readImage(path string) error {
	dirname, filename := filepath.Split(path)
	fsys := os.DirFS(dirname)
	imgIndex := 1 - rs.currentImage
	features, err := jpeg.Decompressor.ReadFile(rs.decompressor, fsys, filename, &rs.images[imgIndex])
	if err != nil {
		return err
	}
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	rs.currentImage = imgIndex
	rs.newestData = frame{
		src:      &rs.images[rs.currentImage],
		features: features,
	}
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
	rs.images[0].Free()
	rs.images[1].Free()
}

func New(logger *zap.Logger, root string) (*Source, error) {
	return &Source{root: root}, nil
}
