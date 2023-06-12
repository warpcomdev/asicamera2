package main

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/warpcomdev/asicamera2/internal/driver/camera"
	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
)

var (
	cameras = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "asicamera_cameras",
			Help: "Number of cameras connected",
		},
		[]string{
			"cameraID",
		})
)

// alert on USB disconnection
func monitorUSB(ctx context.Context, logger servicelog.Logger, config Config, proxy *serverProxy) {
	timer := time.NewTimer(0)
	usbDetected := false // true if usb cammera has been detected once
	usbMissing := false  // True if USB camera has gone from detected to missing
	alertName := "usb_connection"
	usbID := fmt.Sprintf("%s_%s_%s", config.CameraID, alertName, time.Now().Format(time.RFC3339))
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			connectedCameras, err := camera.ASIGetNumOfConnectedCameras()
			if err != nil {
				connectedCameras = 0
				logger.Error("failed to get number of connected cameras", servicelog.Error(err))
			}
			if connectedCameras == 0 && (usbDetected || !usbMissing) {
				logger.Error("No USB camera detected")
				proxy.SendAlert(ctx, usbID, alertName, "error", "No USB camera detected")
				usbDetected = false
				usbMissing = true
			}
			if connectedCameras > 0 {
				usbDetected = true
				if usbMissing {
					logger.Info("USB camera detected")
					proxy.ClearAlert(ctx, usbID)
					// Update usbID so next time there is a disconnection,
					// it will be a new alert
					usbID = fmt.Sprintf("%s_%s_%s", config.CameraID, alertName, time.Now().Format(time.RFC3339))
					usbMissing = false
				}
			}
			cameras.WithLabelValues(config.CameraID).Set(float64(connectedCameras))
			timer.Reset(1 * time.Minute)
		}
	}
}
