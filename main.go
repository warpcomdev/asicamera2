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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/warpcomdev/asicamera2/internal/fakesource"
	"github.com/warpcomdev/asicamera2/internal/jpeg"
	"github.com/warpcomdev/asicamera2/internal/mjpeg"
)

var (
	startMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "start",
		Help: "Start timestamp of the app (unix)",
	})

	infoMetric = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "info",
			Help: "Service info",
		},
		[]string{
			"start",
			"libversion",
		},
	)
)

func abort(funcname string, err error) {
	panic(fmt.Sprintf("%s failed: %v", funcname, err))
}

type fakeSessionManager struct {
	Pool   *jpeg.Pool
	Farm   *jpeg.Farm
	Buffer *jpeg.Buffer
	Source *fakesource.Source
}

type fakeSession struct {
	session    *jpeg.Session
	cancelFunc func()
}

func (f fakeSession) Next(frameNumber uint64) (*jpeg.Frame, uint64, jpeg.FrameStatus) {
	return f.session.Next(frameNumber)
}

func (m fakeSessionManager) Acquire() mjpeg.Session {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return fakeSession{
		session:    m.Buffer.Session(ctx, m.Source, m.Source.Features, m.Pool, m.Farm),
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

	// Register startup metrics
	startTime := time.Now()
	startMetric.Set(float64(startTime.Unix()))
	infoMetric.WithLabelValues(
		startTime.Format(time.RFC3339),
		C.GoString(apiVersion),
	).Set(1)

	fs, err := fakesource.New(os.DirFS("."), os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	frames_per_second := 15
	fsm := fakeSessionManager{
		Pool:   jpeg.NewPool(frames_per_second, fs.Features),
		Farm:   jpeg.NewFarm(8, frames_per_second, jpeg.TJSAMP_420, 95, 0),
		Buffer: jpeg.NewBuffer(3*frames_per_second, fs.Features),
		Source: fs,
	}
	go fsm.Source.Run(context.Background(), frames_per_second)

	http.Handle("/mjpeg", mjpeg.Handler(fsm))
	http.Handle("/metrics", promhttp.Handler())

	fmt.Println("Listening on port :8080")
	srv := &http.Server{
		Addr:           ":8080",
		Handler:        http.DefaultServeMux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   7 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(srv.ListenAndServe())
}
