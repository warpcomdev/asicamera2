package jpeg

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	compressionTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "compression_time",
			Help: "JPEG Compression time (seconds)",
			Buckets: []float64{
				0.010, 0.030, 0.060, 0.120, 0.250, 0.500, 1.000, 2.500,
			},
		},
		[]string{"camera"},
	)

	compressedSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "compressed_size",
			Help: "Size of compressed frames (bytes)",
			Buckets: []float64{
				16384, 65535, 262144, 524288, 1048576, 2097152, 4194304,
			},
		},
		[]string{"camera"},
	)
)

// SrcFrame represents a frame from the source
type SrcFrame interface {
	Buffer() *Image // Buffer pased in to source.Next`, will be returned to freelist.
	// Compress the image into *target*
	Compress(compressor Compressor, target *Image) (JpegFeatures, error)
}

// Source of frames
type Source interface {
	Name() string                                           // identifies the camera name
	Next(ctx context.Context, img *Image) (SrcFrame, error) // get next frame
}

// FrameFactory knows hoy to generate a SrcFrame from an Image
type FrameCompressor struct {
	Subsampling Subsampling
	Quality     int
	Flags       int
}

type RawFrame struct {
	srcFrame    *Image
	camera      string
	subsampling Subsampling
	quality     int
	flags       int
	features    RawFeatures
}

func (f *FrameCompressor) Frame(camera string, img *Image, features RawFeatures) RawFrame {
	return RawFrame{
		srcFrame:    img,
		camera:      camera,
		subsampling: f.Subsampling,
		quality:     f.Quality,
		flags:       f.Flags,
		features:    features,
	}
}

func (f RawFrame) Buffer() *Image {
	return f.srcFrame
}

func (f RawFrame) Compress(compressor Compressor, target *Image) (JpegFeatures, error) {
	start := time.Now()
	feat, err := compressor.Compress(
		f.srcFrame,
		f.features,
		target,
		f.subsampling,
		f.quality,
		f.flags|TJFLAG_NOREALLOC,
	)
	if err != nil {
		compressionTime.WithLabelValues(f.camera).Observe(float64(time.Since(start) / time.Second))
		compressedSize.WithLabelValues(f.camera).Observe(float64(target.Size()))
	}
	return feat, err
}
