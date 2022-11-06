package jpeg

import "context"

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
type FrameFactory struct {
	Subsampling Subsampling
	Quality     int
	Flags       int
}

type RawFrame struct {
	srcFrame    *Image
	subsampling Subsampling
	quality     int
	flags       int
	features    RawFeatures
}

func (f *FrameFactory) Frame(img *Image, features RawFeatures) RawFrame {
	return RawFrame{
		srcFrame:    img,
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
	return compressor.Compress(
		f.srcFrame,
		f.features,
		target,
		f.subsampling,
		f.quality,
		f.flags|TJFLAG_NOREALLOC,
	)
}
