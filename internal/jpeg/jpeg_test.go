package jpeg

import (
	"os"
	"testing"
)

func BenchmarkDecompress(b *testing.B) {
	d := NewDecompressor()
	defer d.Free()

	img := &Image{}
	defer img.Free()
	feat, err := d.ReadFile(os.DirFS("."), "samples/samples.jpg", img)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	out := &Image{}
	for n := 0; n < b.N; n++ {
		if _, err := d.Decompress(img, feat, out, PF_RGBA, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompress(b *testing.B) {
	d := NewDecompressor()
	defer d.Free()

	img := &Image{}
	defer img.Free()
	feat, err := d.ReadFile(os.DirFS("."), "samples/samples.jpg", img)
	if err != nil {
		b.Fatal(err)
	}
	out := &Image{}
	defer out.Free()
	rawFmt, err := d.Decompress(img, feat, out, PF_RGBA, 0)
	if err != nil {
		b.Fatal(err)
	}

	c := NewCompressor()
	defer c.Free()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		if _, err := c.Compress(out, rawFmt, img, feat.Subsampling, 95, 0); err != nil {
			b.Fatal(err)
		}
	}
}
