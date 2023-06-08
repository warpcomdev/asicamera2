package camera

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	asiCameraInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "asi_camera_info",
			Help: "Description of the camera properties",
		},
		[]string{
			"Index",
			"SerialNumber",
			"Name",
			"CameraID",
			"MaxHeight",
			"MaxWidth",
			"IsColorCam",
			"BayerPattern",
			"SupportedBins",
			"SupportedVideoFormat",
			"PixelSize",
			"MechanicalShutter",
			"ST4Port",
			"IsCoolerCam",
			"IsUSB3Host",
			"IsUSB3Camera",
			"ElecPerADU",
			"BitDepth",
			"IsTriggerCam",
		},
	)
)

type ASICamera struct {
	ASI_CAMERA_INFO
	SerialNumber string
	mutex        sync.Mutex
	isOpen       bool
	lastOpen     time.Time
	waiting      int
	join, done   chan struct{}
}

// Opens the connection to ASI camera, if not already open
func (c *ASICamera) Open() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.waiting <= 0 {
		// The camera was forced close and the autoClose thread has exited.
		// forbid open in this state
		return ASI_ERROR_CAMERA_CLOSED
	}
	now := time.Now()
	if !c.isOpen {
		if err := asiOpenCamera(c.CameraID, c.SerialNumber); err != nil {
			return err
		}
		c.isOpen = true
	}
	c.waiting += 1
	c.lastOpen = now
	return nil
}

// Done with the connection, can be closed if no one else interested
func (c *ASICamera) Done() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.waiting -= 1
	if !c.isOpen { // Cancelled during job
		if c.waiting == 0 {
			close(c.done)
		}
	}
}

// New ASICamera handler
func New(index int, autoClose time.Duration) (*ASICamera, error) {
	info := &ASICamera{}
	var err error
	info.ASI_CAMERA_INFO, err = asiGetCameraProperty(index)
	if err != nil {
		return nil, err
	}
	info.SerialNumber = strconv.Itoa(info.CameraID)
	info.waiting = 1 // So nobody tries to close channels yet
	// Get first serial number
	if err := info.Open(); err != nil {
		return nil, err
	}
	defer info.Done()
	info.SerialNumber, err = info.ASIGetSerialNumber()
	if err != nil {
		return nil, err
	}
	// Setup the autocloser
	info.join = make(chan struct{})
	info.done = make(chan struct{})
	go info.autoClose(autoClose)
	// Create info metrics
	svf := make([]string, 0, len(info.SupportedVideoFormat))
	for _, vf := range info.SupportedVideoFormat {
		svf = append(svf, vf.String())
	}
	sb := make([]string, 0, len(info.SupportedBins))
	for _, b := range info.SupportedBins {
		sb = append(svf, strconv.Itoa(b))
	}
	asiCameraInfo.WithLabelValues(
		strconv.Itoa(index),
		info.SerialNumber,
		info.Name,
		strconv.Itoa(info.CameraID),
		strconv.Itoa(info.MaxHeight),
		strconv.Itoa(info.MaxWidth),
		info.IsColorCam.String(),
		info.BayerPattern.String(),
		strings.Join(sb, " "),
		strings.Join(svf, " "),
		fmt.Sprintf("%f", info.PixelSize),
		info.MechanicalShutter.String(),
		info.ST4Port.String(),
		info.IsCoolerCam.String(),
		info.IsUSB3Host.String(),
		info.IsUSB3Camera.String(),
		fmt.Sprintf("%f", info.ElecPerADU),
		strconv.Itoa(info.BitDepth),
		info.IsTriggerCam.String(),
	).Set(1)
	return info, nil
}

// Join ASICamera handler.
func (c *ASICamera) Join() error {
	err := func() error { // to defer
		c.mutex.Lock()
		defer c.mutex.Unlock()
		close(c.join)
		if c.isOpen {
			if err := asiCloseCamera(c.CameraID, c.SerialNumber); err != nil {
				return err
			}
			c.isOpen = false
		}
		return nil
	}()
	if err != nil {
		return err
	}
	// Wait for all users of the camera to exit
	<-c.done
	return nil
}

// autoClose camera after inactivity
func (c *ASICamera) autoClose(autoClose time.Duration) {
	var closeErr error
	for {
		select {
		case <-time.After(autoClose):
			func() { // to defer
				c.mutex.Lock()
				defer c.mutex.Unlock()
				// If it's just me waiting, close the connection
				if c.isOpen && c.waiting == 1 {
					if time.Since(c.lastOpen) > autoClose {
						closeErr = asiCloseCamera(c.CameraID, c.SerialNumber)
						c.isOpen = false
					}
				}
				// Retry closing if it failed
				if !c.isOpen && (errors.Is(closeErr, ASI_ERROR_TIMEOUT) || errors.Is(closeErr, ASI_ERROR_VIDEO_MODE_ACTIVE)) {
					closeErr = asiCloseCamera(c.CameraID, c.SerialNumber)
				}
			}()
			break
		case <-c.join:
			func() { // to defer
				c.mutex.Lock()
				defer c.mutex.Unlock()
				c.waiting -= 1
				if c.waiting == 0 {
					close(c.done)
				}
			}()
			return
		}
	}
}

func (c *ASICamera) ASIGetControlCaps() ([]ASI_CONTROL_CAPS, error) {
	return asiGetControlCaps(c.CameraID, c.SerialNumber)
}

func (c *ASICamera) ASIGetControlValue(controlType ASI_CONTROL_TYPE) (plValue int, pbAuto ASI_BOOL, err error) {
	return asiGetControlValue(c.CameraID, c.SerialNumber, controlType)
}

func (c *ASICamera) ASIGetDroppedFrames() (int, error) {
	return asiGetDroppedFrames(c.CameraID, c.SerialNumber)
}

func (c *ASICamera) ASIGetGainOffset() (pOffset_HighestDR, pOffset_UnityGain, pGain_LowestRN, pOffset_LowestRN int, err error) {
	return asiGetGainOffset(c.CameraID, c.SerialNumber)
}

// Exported as a "keepAlive" function.
// If the SN stops matching c.SerialNumber, the camera has changed.
func (c *ASICamera) ASIGetSerialNumber() (string, error) {
	sn, err := asiGetSerialNumber(c.CameraID)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sn[0:8]), nil
}

// force close the camera if some function failed and we want to force retry
func (c *ASICamera) ForceClose() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.isOpen {
		c.isOpen = false
		return asiCloseCamera(c.CameraID, c.SerialNumber)
	}
	return nil
}
