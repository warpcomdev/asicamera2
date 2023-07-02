package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"path/filepath"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/kardianos/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/warpcomdev/asicamera2/internal/driver/camera"
	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
)

var (
	startMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "asicamera_start",
		Help: "Start timestamp of the app (unix)",
	})

	serviceStartMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "asicamera_service_start",
		Help: "Start timestamp of the service (unix)",
	})

	serviceStopMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "asicamera_service_stop",
		Help: "Stop timestamp of the service (unix)",
	})

	statusMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "asicamera_service_status",
		Help: "Service status",
	})

	infoMetric = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "asicamera_service_info",
			Help: "Service info",
		},
		[]string{
			"start",
			"libversion",
		},
	)
)

type program struct {
	Logger servicelog.Logger
	Config Config
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
	serviceStartMetric.SetToCurrentTime()
	statusMetric.Set(1)
	go func() {
		defer serviceStopMetric.SetToCurrentTime()
		defer statusMetric.Set(0)
		p.Run(ctx)
	}()
	return nil
}

func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	p.Logger.Info("stop signal received")
	if p.Cancel != nil {
		cancel := p.Cancel
		p.Cancel = nil
		// Close the service in the background
		wait := make(chan struct{})
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
	//Caution with absolute timeouts! mjpeg hander is streaming
	//We can use them because mpeghandler implements Hijack to fix
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", p.Config.Port),
		Handler:        mux,
		ReadTimeout:    time.Duration(p.Config.ReadTimeoutSeconds) * time.Second,
		WriteTimeout:   time.Duration(p.Config.WriteTimeoutSeconds) * time.Second,
		MaxHeaderBytes: p.Config.MaxHeaderBytes,
	}
	apiServer := p.Config.Server(p.Logger)
	var wg sync.WaitGroup
	defer wg.Wait()
	// Launch the HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer srv.Close()
			<-ctx.Done()
		}()
		srv.ListenAndServe()
	}()
	// launch the folder watcher
	wg.Add(1)
	go func() {
		defer wg.Done()
		watchMedia(ctx, p.Logger, p.Config, apiServer)
	}()
}

func main() {
	svcConfig := &service.Config{
		Name:        "AsiCameraDriver",
		DisplayName: "ASI Camera image upload driver",
		Description: "Upload ASI camera images to backend service",
	}

	var configPath string
	flag.StringVar(&configPath, "c", "C:\\asicamera\\config.toml", "path to config file")
	flag.Parse()

	configPath, err := filepath.Abs(configPath)
	if err != nil {
		panic(err)
	}

	// Load config
	var config Config
	_, err = toml.DecodeFile(configPath, &config)
	if err != nil {
		panic(err)
	}
	if err := config.Check(configPath); err != nil {
		panic(err)
	}

	prg := &program{
		Config: config,
	}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal("new service failed", err)
	}

	// Setup logging
	errs := make(chan error, 16)
	go func() {
		for {
			err := <-errs
			if err != nil {
				log.Print(err)
			}
		}
	}()
	rootLogger, err := s.Logger(errs)
	if err != nil {
		log.Fatal(err)
	}
	logger, err := servicelog.New(rootLogger, config.LogFolder, config.Debug)
	if err != nil {
		panic(err)
	}
	prg.Logger = logger
	defer logger.Sync()

	anonimizedConfig := config
	anonimizedConfig.ApiKey = "********"
	logger.Info("config", servicelog.Any("config", anonimizedConfig))

	// Get SDK version
	apiVersion, err := camera.ASIGetSDKVersion()
	if err != nil {
		logger.Fatal("Failed to get SDK version", servicelog.Error(err))
		return
	}
	logger.Info("ASICamera2 SDK version", servicelog.String("apiVersion", apiVersion))

	// Register startup metrics
	startTime := time.Now()
	startMetric.Set(float64(startTime.Unix()))
	infoMetric.WithLabelValues(
		startTime.Format(time.RFC3339),
		apiVersion,
	).Set(1)

	args := flag.Args()
	if len(args) > 0 {
		err = service.Control(s, args[0])
		if err != nil {
			logger.Fatal("service control failed", servicelog.Error(err))
		}
		return
	}

	logger.Info("starting service manager")
	err = s.Run()
	if err != nil {
		logger.Error("run failed", servicelog.Error(err))
	}
}
