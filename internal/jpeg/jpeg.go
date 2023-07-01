package jpeg

/*
#cgo CFLAGS:   -I${SRCDIR}/../../include
#cgo LDFLAGS:  -L${SRCDIR}/../../lib -l:libjpeg.a -l:libturbojpeg.a -l:libjpeg.dll.a -l:libturbojpeg.dll.a
#include "turbojpeg.h"

int bytes_per_pixel(int mode) {
	return tjPixelSize[mode];
}
*/
import "C"

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"unsafe"
)

type ColorSpace int  // ColorSpace of the image (see TJCS in turbojpeg.h)
type PixelFormat int // PixelFormat of the raw image (See TJPF in turbojpeg.h)
type Subsampling int // Subsampling of the jpeg image (See TJSAMP in turbojpeg.h)

const (
	PF_RGBA PixelFormat = C.TJPF_RGBA // Common pixel format for golang.Image
	PF_RGB  PixelFormat = C.TJPF_RGB  // Common pixel format for ASICamera
)

const TJFLAG_NOREALLOC = C.TJFLAG_NOREALLOC

const (
	TJSAMP_444  Subsampling = C.TJSAMP_444
	TJSAMP_422  Subsampling = C.TJSAMP_422
	TJSAMP_420  Subsampling = C.TJSAMP_420
	TJSAMP_GRAY Subsampling = C.TJSAMP_GRAY
	TJSAMP_440  Subsampling = C.TJSAMP_440
	TJSAMP_411  Subsampling = C.TJSAMP_411
)

// Image buffer. Can be a jpeg or a raw image
type Image struct {
	buffer  *C.uchar // Buffer as unsigned char * (turbojpeg API format)
	imgsize int      // size of the image in the buffer
}

// Size currently allocated to the image
func (img *Image) Size() int {
	return img.imgsize
}

// Slice returns a reference to the image as a byte slice.
// This should be consumed before encoding or decoding the image
// again, since these operations can alter the underlying buffer.
func (img *Image) Slice() []byte {
	return unsafe.Slice((*byte)(img.buffer), img.imgsize)
}

// Alloc a buffer with the given capacity in bytes
func (img *Image) Alloc(size int) error {
	log.Printf("Allocating %d bytes for image", size)
	buf := C.tjAlloc(C.int(size))
	if buf == nil {
		return fmt.Errorf("Failed to allocate %d bytes", size)
	}
	img.Free()
	img.buffer = buf
	img.imgsize = size
	return nil
}

// Free the allocated buffer, if any
func (img *Image) Free() {
	if img.imgsize > 0 {
		log.Printf("Releasing image of size %d", img.imgsize)
		C.tjFree(img.buffer)
	}
	img.imgsize = 0
}

// Features shared by jpeg and raw images
type Features struct {
	Width  int // width in pixels
	Height int // heigh in pixels
}

// JpegFeatures exclusive to jpeg images
type JpegFeatures struct {
	Features
	Subsampling Subsampling // Subsampling enum (see turbojpeg.h)
	ColorSpace  ColorSpace  // ColorSpace enum (see turbojpeg.h)
}

// RawFeatures exclusive to raw images
type RawFeatures struct {
	Features
	Format PixelFormat // Pixelformat
}

// Pitch equals width times bytes per pixel
func (r RawFeatures) Pitch() int {
	return int(C.bytes_per_pixel(C.int(r.Format))) * r.Width
}

type Compressor struct {
	handle C.tjhandle
}

type Decompressor struct {
	handle C.tjhandle
}

func handleError(handle C.tjhandle) error {
	return errors.New(C.GoString(C.tjGetErrorStr2(handle)))
}

// newCompressor creates a decompressor with buffer size enough for 1920x1080
func NewCompressor() Compressor {
	return Compressor{
		handle: C.tjInitCompress(),
	}
}

// Free compressor resources
func (c Compressor) Free() {
	C.tjDestroy(c.handle)
}

// newDecompressor creates a decompressor with buffer size enough for 1920x1080
func NewDecompressor() Decompressor {
	return Decompressor{
		handle: C.tjInitDecompress(),
	}
}

// Free decompressor resources
func (d Decompressor) Free() {
	C.tjDestroy(d.handle)
}

// readFile is an utility function to read a jpeg file into a buffer
func (d Decompressor) ReadFile(fsys fs.FS, path string, img *Image) (JpegFeatures, error) {
	if img == nil {
		return JpegFeatures{}, errors.New("img cannot be nil")
	}
	infile, err := fsys.Open(path)
	if err != nil {
		return JpegFeatures{}, err
	}
	info, err := infile.Stat()
	if err != nil {
		return JpegFeatures{}, err
	}
	size := int(info.Size())
	if img.imgsize < size {
		img.Alloc(size)
	}
	read, err := infile.Read(img.Slice())
	if err != nil {
		return JpegFeatures{}, err
	}
	if read < int(size) {
		return JpegFeatures{}, fmt.Errorf("failed to read %d bytes of file %s, eof at %d", size, path, read)
	}
	img.imgsize = read
	var width, height, jpegSubsamp, jpegColorspace C.int
	res := C.tjDecompressHeader3(d.handle, img.buffer, C.ulong(img.imgsize), &width, &height, &jpegSubsamp, &jpegColorspace)
	if res != 0 {
		return JpegFeatures{}, fmt.Errorf("failed to decode image %s with imagesize: %d: %w", path, img.imgsize, handleError(d.handle))
	}
	// Return features
	return JpegFeatures{
		Features: Features{
			Width:  int(width),
			Height: int(height),
		},
		Subsampling: Subsampling(jpegSubsamp),
		ColorSpace:  ColorSpace(jpegColorspace),
	}, nil
}

// decompress the input buffer
func (d Decompressor) Decompress(input *Image, jpegFeat JpegFeatures, output *Image, format PixelFormat, flags int) (RawFeatures, error) {
	if input == nil {
		return RawFeatures{}, errors.New("input cannot be nil")
	}
	if output == nil {
		return RawFeatures{}, errors.New("output cannot be nil")
	}
	rawFeat := RawFeatures{Features: jpegFeat.Features, Format: format}
	pitch := rawFeat.Pitch()
	rawSize := pitch * rawFeat.Height
	if output.imgsize < rawSize {
		if err := output.Alloc(rawSize); err != nil {
			return RawFeatures{}, err
		}
	}
	code := C.tjDecompress2(d.handle, input.buffer, C.ulong(input.imgsize), output.buffer, C.int(rawFeat.Width), C.int(pitch), C.int(rawFeat.Height), C.int(rawFeat.Format), C.int(flags))
	if code != 0 {
		return RawFeatures{}, handleError(d.handle)
	}
	return rawFeat, nil
}

// Compress the input buffer
func (c *Compressor) Compress(input *Image, rawFeat RawFeatures, output *Image, subsamp Subsampling, quality int, flags int) (JpegFeatures, error) {
	buffer, bufsize := output.buffer, C.ulong(output.imgsize)
	code := C.tjCompress2(c.handle, input.buffer, C.int(rawFeat.Width), C.int(rawFeat.Pitch()), C.int(rawFeat.Height), C.int(rawFeat.Format), &buffer, &bufsize, C.int(subsamp), C.int(quality), C.int(flags))
	if code != 0 {
		return JpegFeatures{}, handleError(c.handle)
	}
	output.buffer = buffer
	output.imgsize = int(bufsize)
	return JpegFeatures{
		Features:    rawFeat.Features,
		Subsampling: Subsampling(subsamp),
		ColorSpace:  0, // FIXME: any way to find out the colorspace of the generated image?
	}, nil
}

// WriteFile saves the compressed image to the given path
func (img *Image) WriteFile(path string) error {
	infile, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := infile.Write(img.Slice()); err != nil {
		return err
	}
	return nil
}
