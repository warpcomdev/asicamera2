package main

/*
#cgo CFLAGS: -I${SRCDIR}/include
#cgo LDFLAGS: -L${SRCDIR}/lib -l:ASICamera2.lib
#include "ASICamera2.h"
*/
import "C"

import (
	"fmt"
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
}
