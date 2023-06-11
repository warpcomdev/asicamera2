package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/kardianos/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/warpcomdev/asicamera2/internal/driver/camera"
	"go.uber.org/zap"
)

var (
	startMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "start",
		Help: "Start timestamp of the app (unix)",
	})

	serviceStartMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "service_start",
		Help: "Start timestamp of the service (unix)",
	})

	serviceStopMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "service_stop",
		Help: "Stop timestamp of the service (unix)",
	})

	statusMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "status",
		Help: "Service status",
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
	Logger *zap.Logger
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

	// Load config
	var config Config
	_, err := toml.DecodeFile(configPath, &config)
	if err != nil {
		panic(err)
	}
	if err := config.Check(); err != nil {
		panic(err)
	}

	var logger *zap.Logger
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

	anonimizedConfig := config
	anonimizedConfig.ApiKey = "********"
	logger.Info("config", zap.Any("config", anonimizedConfig))

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
	args := flag.Args()
	if len(args) > 1 {
		err = service.Control(s, args[1])
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
