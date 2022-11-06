package jpeg

import (
	"bytes"
	imagejpeg "image/jpeg"
	"io"
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

func BenchmarkDecompressBuiltin(b *testing.B) {
	infile, err := os.Open("samples/samples.jpg")
	if err != nil {
		b.Fatal(err)
	}
	buf, err := io.ReadAll(infile)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if _, err := imagejpeg.Decode(bytes.NewReader(buf)); err != nil {
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

func BenchmarkCompressBuiltin(b *testing.B) {
	infile, err := os.Open("samples/samples.jpg")
	if err != nil {
		b.Fatal(err)
	}
	buf, err := io.ReadAll(infile)
	if err != nil {
		b.Fatal(err)
	}
	image, err := imagejpeg.Decode(bytes.NewReader(buf))
	w := bytes.NewBuffer(make([]byte, 0, len(buf)))

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		w.Reset()
		if imagejpeg.Encode(w, image, &imagejpeg.Options{Quality: 95}); err != nil {
			b.Fatal(err)
		}
	}
}
