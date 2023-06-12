package dirsource

import (
	"context"
	"errors"
	"io/fs"
	"time"

	"github.com/warpcomdev/asicamera2/internal/driver/jpeg"
	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
)

// Source of frames
type Source struct {
	SourceName string
	RawImage   *jpeg.Image
	Features   jpeg.RawFeatures
	Factory    jpeg.FrameCompressor
	Stream     chan *jpeg.Image
	Offset     int
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
				break
			default:
			}
			break
		case <-ctx.Done():
			return
		}
	}
}

func (s *Source) Next(ctx context.Context, img *jpeg.Image) (jpeg.SrcFrame, error) {
	select {
	case <-ctx.Done():
		return nil, errors.New("context cancelled")
	case srcImg := <-s.Stream:
		if img.Size() < srcImg.Size() {
			img.Free()
			img.Alloc(srcImg.Size())
		}
		srcSlice := srcImg.Slice()
		dstSlice := img.Slice()
		copy(dstSlice, srcSlice)
	}
	return s.Factory.Frame(s.SourceName, img, s.Features), nil
}

// Name implements jpeg.Source
func (s *Source) Name() string {
	return s.SourceName
}

// Source of frames
type ResumableSource struct {
	Source
	FramesPerSecond int
	CancelFunc      func()
}

func (rs *ResumableSource) Start(servicelog.Logger) error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	rs.CancelFunc = cancelFunc
	go rs.Run(ctx, rs.FramesPerSecond)
	return nil
}

func (rs *ResumableSource) Stop() {
	rs.CancelFunc()
}

func New(fsys fs.FS, path string, fps int, factory jpeg.FrameCompressor) (*ResumableSource, error) {
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

	return &ResumableSource{
		Source: Source{
			SourceName: path,
			RawImage:   out,
			Factory:    factory,
			Features:   features,
			Stream:     make(chan *jpeg.Image),
		},
		FramesPerSecond: fps,
	}, nil
}
