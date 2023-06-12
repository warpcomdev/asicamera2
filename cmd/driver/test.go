package main

import (
	_ "net/http/pprof"
)

/*
type fakeSessionManager struct {
	Manager *jpeg.SessionManager
}

func (m fakeSessionManager) Acquire(logger servicelog.Logger) (mjpeg.Session, error) {
	return m.Manager.Acquire(logger)
}

func (m fakeSessionManager) Done() {
	m.Manager.Done()
}

func test() {
	fmt.Println("Entering program")

	logger, err := servicelog.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	apiVersion, err := camera.ASIGetSDKVersion()
	if err != nil {
		panic(err)
	}
	fmt.Printf("ASICamera2 SDK version %s\n", apiVersion)

	connectedCameras, err := camera.ASIGetNumOfConnectedCameras()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Number of connected cameras %d\n", connectedCameras)

	if connectedCameras > 0 {
		info, err := camera.New(0, 5*time.Minute)
		if err != nil {
			panic(err)
		}
		defer info.Join()
		data, err := json.MarshalIndent(info.ASI_CAMERA_INFO, "", "  ")
		if err != nil {
			panic(err)
		}
		fmt.Println(string(data))
		caps, err := info.ASIGetControlCaps()
		if err != nil {
			panic(err)
		}
		data, err = json.MarshalIndent(caps, "", "  ")
		if err != nil {
			panic(err)
		}
		go info.Monitor(context.Background(), logger, 30*time.Second)
		fmt.Println(string(data))
	}

	frames_per_second := 15
	compressor_threads := 8

	if len(os.Args) > 1 {
		// commonSource, err := dirsource.New(os.DirFS("."), os.Args[1], frames_per_second, jpeg.FrameFactory{
		// 	Subsampling: jpeg.TJSAMP_420,
		// 	Quality:     95,
		// 	Flags:       0,
		// })
		commonSource, err := dirsource.New(logger, os.Args[1], regexp.MustCompile(`\d{4}-\d{2}-\d{2}`))
		if err != nil {
			log.Fatal(err)
		}

		feat := jpeg.RawFeatures{
			Features: jpeg.Features{
				Width:  1920,
				Height: 1080,
			},
			Format: jpeg.PF_RGBA,
		}
		imgSize := feat.Pitch() * feat.Height
		pool := jpeg.NewPool(frames_per_second, imgSize)
		defer pool.Free()
		farm := jpeg.NewFarm(logger, compressor_threads, frames_per_second)
		defer farm.Stop()

		firstCamera := true
		for idx, camera := range []string{"cam0", "cam1"} {
			fs := commonSource
			pipeline := jpeg.New(pool, farm, 3*frames_per_second, imgSize)
			defer pipeline.Join()
			manager := pipeline.Manage(fs)

			mjpeg_handler := mjpeg.Handler(logger, fakeSessionManager{Manager: manager})
			http.Handle("/mjpeg/"+strconv.Itoa(idx), mjpeg_handler)
			http.Handle("/mjpeg/"+camera, mjpeg_handler)

			jpeg_handler := jpeg.Handler(logger, manager)
			http.Handle("/jpeg/"+strconv.Itoa(idx), jpeg_handler)
			http.Handle("/jpeg/"+camera, jpeg_handler)

			if firstCamera {
				firstCamera = false
				http.Handle("/mjpeg", mjpeg_handler)
				http.Handle("/jpeg", jpeg_handler)
			}
		}

	}

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/debug", http.DefaultServeMux)

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
*/
