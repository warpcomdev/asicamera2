package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/kardianos/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/warpcomdev/asicamera2/internal/driver/camera"
	"github.com/warpcomdev/asicamera2/internal/driver/camera/backend"
	"github.com/warpcomdev/asicamera2/internal/driver/watcher"
	"go.uber.org/zap"
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

type program struct {
	Config *Config
	Logger *zap.Logger
	Cancel func()
}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	p.Logger.Info("start signal received")
	if p.Cancel != nil {
		if err := p.Stop(s); err != nil {
			return err
		}
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	p.Cancel = cancelFunc
	go p.Run(ctx)
	return nil
}

func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	p.Logger.Info("stop signal received")
	if p.Cancel != nil {
		cancel := p.Cancel
		p.Cancel = nil
		// Close the service in the background
		wait := make(chan struct{}, 0)
		go func() {
			defer close(wait)
			cancel()
		}()
		// Wait up to two seconds for cancellation
		select {
		case <-wait:
			break
		case <-time.After(2 * time.Second):
			break
		}
	}
	return nil
}

func (p *program) Run(ctx context.Context) {
	mux := &http.ServeMux{}
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/debug", http.DefaultServeMux)
	//Cannot set absolute timeout because mjpeg hander is streaming
	//Must implement Hijack to fix
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", p.Config.Port),
		Handler:        mux,
		ReadTimeout:    time.Duration(p.Config.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(p.Config.WriteTimeout) * time.Second,
		MaxHeaderBytes: p.Config.MaxHeaderBytes,
	}
	timer := backoff.NewExponentialBackOff()
	maxBackoff := 5 * time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		default:
			start := time.Now()
			p.Logger.Info("server started")
			dualContext, cancel := context.WithCancel(ctx)
			var (
				wg      sync.WaitGroup
				apiErr  error
				httpErr error
			)
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer srv.Close()
				<-dualContext.Done()
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer cancel()
				apiServer, err := backend.New(p.Logger)
				if err != nil {
					apiErr = err
					return
				}
				folder, err := apiServer.Folder(dualContext)
				if err != nil {
					apiErr = err
					return
				}
				watch, err := watcher.New(p.Logger, p.Config.HistoryFolder, apiServer, folder, p.Config.FileTypes(), time.Duration(p.Config.MonitorFor)*time.Minute)
				if err != nil {
					apiErr = err
					return
				}
				apiErr = watch.Watch(dualContext)
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer cancel()
				httpErr = srv.ListenAndServe()
			}()
			wg.Wait()
			stop := time.Now()
			if (apiErr == nil || errors.Is(apiErr, context.Canceled)) &&
				(httpErr == nil || errors.Is(httpErr, http.ErrServerClosed)) {
				p.Logger.Info("server stopped")
				return
			}
			// If the service was up for a decent amount of time, reset the backoff
			if stop.Sub(start) > 5*time.Second {
				timer.Reset()
			}
			duration := timer.NextBackOff()
			if duration == backoff.Stop {
				duration = maxBackoff
			}
			err := errors.Join(apiErr, httpErr)
			p.Logger.Error("server failed, retrying", zap.Error(err), zap.Duration("backoff", duration))
			<-time.After(duration)
		}
	}
}

func main() {
	svcConfig := &service.Config{
		Name:        "AsiCameraDriver",
		DisplayName: "ASI Camera image upload driver",
		Description: "Upload ASI camera images to backend service",
	}

	config := newConfig()
	var (
		logger *zap.Logger
		err    error
	)
	if config.Debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	// Avoid stack traces below panic level
	logger = logger.WithOptions(zap.AddStacktrace(zap.DPanicLevel))
	// Set logging level
	defer logger.Sync()

	// Get SDK version
	apiVersion, err := camera.ASIGetSDKVersion()
	if err != nil {
		logger.Fatal("Failed to get SDK version", zap.Error(err))
		return
	}
	logger.Info("ASICamera2 SDK version", zap.String("apiVersion", apiVersion))

	// Register startup metrics
	startTime := time.Now()
	startMetric.Set(float64(startTime.Unix()))
	infoMetric.WithLabelValues(
		startTime.Format(time.RFC3339),
		apiVersion,
	).Set(1)

	prg := &program{
		Logger: logger,
		Config: config,
	}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		logger.Fatal("new service failed", zap.Error(err))
	}
	if len(os.Args) > 1 {
		err = service.Control(s, os.Args[1])
		if err != nil {
			logger.Fatal("service control failed", zap.Error(err))
		}
		return
	}

	logger.Info("starting service manager")
	err = s.Run()
	if err != nil {
		logger.Error("run failed", zap.Error(err))
	}
}
