package camera

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	asiControlCaps = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "asi_control_caps",
			Help: "Capabilities of the camera",
		},
		[]string{
			"camera",
			"name",
			"description",
			"maxValue",
			"minValue",
			"defaultValue",
			"isAutoSupported",
			"isWritable",
			"controlType",
		},
	)

	asiCameraUp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "asi_camera_up",
			Help: "ASI Camera metrics keepalive",
		},
		[]string{"camera"},
	)

	controlTypeOverclock = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "control_type_overclock",
			Help: "Value of ASI_CONTROL_TYPE ASI_OVERCLOCK",
		},
		[]string{"camera"},
	)

	controlTypeTemperature = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "control_type_temperature",
			Help: "Value of ASI_CONTROL_TYPE ASI_TEMPERATURE",
		},
		[]string{"camera"},
	)

	controlTypeTargetTemp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "control_type_target_temp",
			Help: "Value of ASI_CONTROL_TYPE ASI_TARGET_TEMP",
		},
		[]string{"camera"},
	)

	controlTypeCoolerPowerPerc = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "control_type_cooler_power_perc",
			Help: "Value of ASI_CONTROL_TYPE ASI_COOLER_POWER_PERC",
		},
		[]string{"camera"},
	)

	controlTypeFanOn = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "control_type_fan_on",
			Help: "Value of ASI_CONTROL_TYPE ASI_FAN_ON",
		},
		[]string{"camera"},
	)

	controlTypeAntiDewHeater = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "control_type_anti_dew_heater",
			Help: "Value of ASI_CONTROL_TYPE ANTI_DEW_HEATER",
		},
		[]string{"camera"},
	)
)

func (c *ASICamera) Monitor(ctx context.Context, interval time.Duration) {
	metrics := map[ASI_CONTROL_TYPE]*prometheus.GaugeVec{
		ASI_OVERCLOCK:         controlTypeOverclock,
		ASI_TEMPERATURE:       controlTypeTemperature,
		ASI_TARGET_TEMP:       controlTypeTargetTemp,
		ASI_COOLER_POWER_PERC: controlTypeCoolerPowerPerc,
		ASI_FAN_ON:            controlTypeFanOn,
		ASI_ANTI_DEW_HEATER:   controlTypeAntiDewHeater,
	}
	var supported_metrics []ASI_CONTROL_TYPE
	var currentInterval time.Duration = 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-time.After(currentInterval):
			currentInterval = interval
			func() { // to defer
				if err := c.Open(); err != nil {
					log.Printf("ASICamera::Monitor failed to open: %v", err)
					return
				}
				defer c.Done()
				if supported_metrics == nil {
					// Get all the metrics the camera supports
					all_caps, err := c.ASIGetControlCaps()
					if err != nil {
						log.Printf("ASICamera::Monitor: failed to get caps: %v", err)
						return
					}
					// Only monitor supported metrics
					supported_metrics = make([]ASI_CONTROL_TYPE, 0, len(all_caps))
					for _, metric := range all_caps {
						asiControlCaps.WithLabelValues(
							c.SerialNumber,
							metric.Name,
							metric.Description,
							strconv.Itoa(metric.MaxValue),
							strconv.Itoa(metric.MinValue),
							strconv.Itoa(metric.DefaultValue),
							metric.IsAutoSupported.String(),
							metric.IsWritable.String(),
							metric.ControlType.String(),
						).Set(1)
						if _, ok := metrics[metric.ControlType]; ok {
							supported_metrics = append(supported_metrics, metric.ControlType)
						}
					}
				}
				for _, controlType := range supported_metrics {
					alive := 0
					metric, _, err := asiGetControlValue(c.CameraID, c.SerialNumber, controlType)
					if err != nil {
						log.Printf("ASICamera::Monitor gauge %s failed: %v", controlType.String(), err)
						alive = 0
					} else {
						gauge := metrics[controlType]
						gauge.WithLabelValues(c.SerialNumber).Set(float64(metric))
					}
					asiCameraUp.WithLabelValues(c.SerialNumber).Set(float64(alive))
				}
			}()
		}
	}
}
