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
	Manager *jpeg.SessionManager
}

func (m fakeSessionManager) Acquire() (mjpeg.Session, error) {
	return m.Manager.Acquire()
}

func (m fakeSessionManager) Done() {
	m.Manager.Done()
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

	frames_per_second := 15
	fs, err := fakesource.New(os.DirFS("."), os.Args[1], frames_per_second)
	if err != nil {
		log.Fatal(err)
	}

	pipeline := jpeg.New(frames_per_second, 3*frames_per_second, 8, fs.Features, jpeg.TJSAMP_420, 95, 0)
	manager := pipeline.Manage(fs)
	defer manager.Join()

	http.Handle("/mjpeg", mjpeg.Handler(fakeSessionManager{Manager: manager}))
	http.Handle("/jpeg", jpeg.Handler(manager))
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
