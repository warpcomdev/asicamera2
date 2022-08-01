package main

/*
#cgo CFLAGS:   -I${SRCDIR}/include
#cgo LDFLAGS:  -L${SRCDIR}/lib -l:ASICamera2.lib
#include "ASICamera2.h"
*/
import "C"

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/warpcomdev/asicamera2/internal/fakesource"
	"github.com/warpcomdev/asicamera2/internal/jpeg"
	"github.com/warpcomdev/asicamera2/internal/mjpeg"
)

func abort(funcname string, err error) {
	panic(fmt.Sprintf("%s failed: %v", funcname, err))
}

type fakeSessionManager struct {
	Pipeline *jpeg.Pipeline
	Source   *fakesource.Source
}

type fakeSession struct {
	session    *jpeg.Session
	cancelFunc func()
}

func (f fakeSession) Next(frameNumber uint64) (*jpeg.JpegFrame, uint64, jpeg.FrameStatus) {
	return f.session.Next(frameNumber)
}

func (m fakeSessionManager) Acquire() mjpeg.Session {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return fakeSession{
		session:    m.Pipeline.Session(ctx, m.Source, m.Source.Features),
		cancelFunc: cancelFunc,
	}
}

func (m fakeSessionManager) Release(session mjpeg.Session) {
	fullSession := session.(fakeSession)
	fullSession.cancelFunc()
	fullSession.session.Join()
}

func main() {
	fmt.Println("Entering program")

	apiVersion, err := C.ASIGetSDKVersion()
	if err != nil {
		panic(err)
	}
	fmt.Printf("ASICamera2 SDK version %s\n", C.GoString(apiVersion))

	if len(os.Args) <= 1 {
		return
	}

	fs, err := fakesource.New(os.DirFS("."), os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	frames_per_second := 15
	fsm := fakeSessionManager{
		Pipeline: jpeg.New(frames_per_second, 3*frames_per_second, 8, fs.Features, jpeg.TJSAMP_420, 95, 0),
		Source:   fs,
	}
	go fsm.Source.Run(context.Background(), frames_per_second)
	handler := mjpeg.Handler(fsm)

	http.HandleFunc("/mjpeg", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		if strings.HasPrefix(r.URL.Path, "/mjpeg") {
			handler.ServeHTTP(w, r)
			return
		}
		http.Error(w, "path not found", http.StatusNotFound)
	})
	http.Handle("/metrics", promhttp.Handler())

	fmt.Println("Listening on port :8080")
	//Cannot set absolute timeout because mjpeg hander is streaming
	//Must implement Hijack to fix
	srv := &http.Server{
		Addr:           ":8080",
		Handler:        http.DefaultServeMux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   7 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(srv.ListenAndServe())
	//log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
