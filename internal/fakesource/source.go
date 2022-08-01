package fakesource

import (
	"context"
	"errors"
	"io/fs"
	"time"

	"github.com/warpcomdev/asicamera2/internal/jpeg"
)

// Source of frames
type Source struct {
	RawImage *jpeg.Image
	Features jpeg.RawFeatures
	Stream   chan *jpeg.Image
	Offset   int
}

func New(fsys fs.FS, path string) (*Source, error) {
	d := jpeg.NewDecompressor()
	defer d.Free()

	img := &jpeg.Image{}
	defer img.Free()
	feat, err := d.ReadFile(fsys, path, img)
	if err != nil {
		return nil, err
	}
	out := &jpeg.Image{}
	features, err := d.Decompress(img, feat, out, jpeg.PF_RGBA, 0)
	if err != nil {
		return nil, err
	}

	return &Source{
		RawImage: out,
		Features: features,
		Stream:   make(chan *jpeg.Image),
	}, nil
}

func (s *Source) Run(ctx context.Context, fps int) {
	ticker := time.NewTicker(time.Duration(1000/fps) * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			select {
			case s.Stream <- s.RawImage:
				buff := s.RawImage.Slice()
				pitch := s.Features.Pitch()
				line := make([]byte, pitch)
				// Rotate the image one scan line
				total := len(buff)
				c := copy(line, buff)
				c += copy(buff, buff[pitch:])
				c += copy(buff[total-pitch:], line)
			default:
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Source) Next(ctx context.Context, img *jpeg.Image) error {
	select {
	case <-ctx.Done():
		return errors.New("Context cancelled")
	case srcImg := <-s.Stream:
		if img.Size() < srcImg.Size() {
			img.Free()
			img.Alloc(srcImg.Size())
		}
		srcSlice := srcImg.Slice()
		dstSlice := img.Slice()
		copy(dstSlice, srcSlice)
	}
	return nil
}
