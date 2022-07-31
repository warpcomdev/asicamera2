package main

/*
#cgo CFLAGS:   -I${SRCDIR}/include
#cgo LDFLAGS:  -L${SRCDIR}/lib -l:ASICamera2.lib
#include "ASICamera2.h"
*/
import "C"

import (
	"fmt"
	"log"
	"os"

	"github.com/warpcomdev/asicamera2/internal/jpeg"
)

func abort(funcname string, err error) {
	panic(fmt.Sprintf("%s failed: %v", funcname, err))
}

func main() {
	apiVersion, err := C.ASIGetSDKVersion()
	if err != nil {
		panic(err)
	}
	fmt.Printf("ASICamera2 SDK version %s\n", C.GoString(apiVersion))

	d := jpeg.NewDecompressor()
	defer d.Free()

	encoded := &jpeg.Image{}
	defer encoded.Free()
	feat, err := d.ReadFile(os.DirFS("."), "internal/jpeg/samples/samples.jpg", encoded)
	if err != nil {
		log.Fatal(err)
	}
	decoded := &jpeg.Image{}
	defer decoded.Free()
	rawFmt, err := d.Decompress(encoded, feat, decoded, jpeg.PF_RGBA, 0)
	if err != nil {
		log.Fatal(err)
	}

	c := jpeg.NewCompressor()
	defer c.Free()

	roundtrip := &jpeg.Image{}
	defer roundtrip.Free()

	if _, err := c.Compress(decoded, rawFmt, roundtrip, feat.Subsampling, 95, 0); err != nil {
		log.Fatal(err)
	}
	roundtrip.WriteFile("internal/jpeg/samples/roundtrip.jpeg")
}
