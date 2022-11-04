package camera

/*
#cgo CFLAGS:   -I${SRCDIR}/../../../include
#cgo LDFLAGS:  -L${SRCDIR}/../../../lib -l:ASICamera2.lib

#include <stdlib.h>
#include "ASICamera2.h"

typedef struct property_wrapper {
	ASI_CAMERA_INFO info;
	int retcode;
} property_wrapper;

property_wrapper* camera_properties(int index) {
	property_wrapper* w = (property_wrapper*)malloc(sizeof(property_wrapper));
	w->retcode = ASIGetCameraProperty(&(w->info), index);
	return w;
}

void free_properties(property_wrapper *w) {
	free(w);
}
*/
import "C"
import (
	"fmt"
)

type ASI_BAYER_PATTERN int
const (
	ASI_BAYER_RG ASI_BAYER_PATTERN = C.ASI_BAYER_RG
	ASI_BAYER_BG ASI_BAYER_PATTERN = C.ASI_BAYER_BG
	ASI_BAYER_GR ASI_BAYER_PATTERN = C.ASI_BAYER_GR
	ASI_BAYER_GB ASI_BAYER_PATTERN = C.ASI_BAYER_GB
)
func (p ASI_BAYER_PATTERN) String() string {
	text := map[ASI_BAYER_PATTERN]string{
		ASI_BAYER_RG: "ASI_BAYER_RG",
		ASI_BAYER_BG: "ASI_BAYER_BG",
		ASI_BAYER_GR: "ASI_BAYER_GR",
		ASI_BAYER_GB: "ASI_BAYER_GB",
	}
	return text[p]
}
func (p ASI_BAYER_PATTERN) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", p.String())), nil
}

type ASI_IMG_TYPE int
const (
	ASI_IMG_RAW8 ASI_IMG_TYPE = C.ASI_IMG_RAW8
	ASI_IMG_RGB24 ASI_IMG_TYPE = C.ASI_IMG_RGB24
	ASI_IMG_RAW16 ASI_IMG_TYPE = C.ASI_IMG_RAW16
	ASI_IMG_Y8 ASI_IMG_TYPE = C.ASI_IMG_Y8
	ASI_IMG_END ASI_IMG_TYPE = C.ASI_IMG_END
)
func (p ASI_IMG_TYPE) String() string {
	text := map[ASI_IMG_TYPE]string{
		ASI_IMG_RAW8: "ASI_IMG_RAW8",
		ASI_IMG_RGB24: "ASI_IMG_RGB24",
		ASI_IMG_RAW16: "ASI_IMG_RAW16",
		ASI_IMG_Y8: "ASI_IMG_Y8",
		ASI_IMG_END: "ASI_IMG_END",
	}
	return text[p]
}
func (p ASI_IMG_TYPE) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", p.String())), nil
}

type ASI_GUIDE_DIRECTION int
const (
	ASI_GUIDE_NORTH ASI_GUIDE_DIRECTION = C.ASI_GUIDE_NORTH
	ASI_GUIDE_SOUTH ASI_GUIDE_DIRECTION = C.ASI_GUIDE_SOUTH
	ASI_GUIDE_EAST ASI_GUIDE_DIRECTION = C.ASI_GUIDE_EAST
	ASI_GUIDE_WEST ASI_GUIDE_DIRECTION = C.ASI_GUIDE_WEST
)
func (p ASI_GUIDE_DIRECTION) String() string {
	text := map[ASI_GUIDE_DIRECTION]string{
		ASI_GUIDE_NORTH: "ASI_GUIDE_NORTH",
		ASI_GUIDE_SOUTH: "ASI_GUIDE_SOUTH",
		ASI_GUIDE_EAST: "ASI_GUIDE_EAST",
		ASI_GUIDE_WEST: "ASI_GUIDE_WEST",
	}
	return text[p]
}
func (p ASI_GUIDE_DIRECTION) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", p.String())), nil
}

type ASI_FLIP_STATUS int
const (
	ASI_FLIP_NONE ASI_FLIP_STATUS = C.ASI_FLIP_NONE
	ASI_FLIP_HORIZ ASI_FLIP_STATUS = C.ASI_FLIP_HORIZ
	ASI_FLIP_VERT ASI_FLIP_STATUS = C.ASI_FLIP_VERT
	ASI_FLIP_BOTH ASI_FLIP_STATUS = C.ASI_FLIP_BOTH
)
func (p ASI_FLIP_STATUS) String() string {
	text := map[ASI_FLIP_STATUS]string{
		ASI_FLIP_NONE: "ASI_FLIP_NONE",
		ASI_FLIP_HORIZ: "ASI_FLIP_HORIZ",
		ASI_FLIP_VERT: "ASI_FLIP_VERT",
		ASI_FLIP_BOTH: "ASI_FLIP_BOTH",
	}
	return text[p]
}
func (p ASI_FLIP_STATUS) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", p.String())), nil
}

type ASI_CAMERA_MODE int
const (
	ASI_MODE_NORMAL ASI_CAMERA_MODE = C.ASI_MODE_NORMAL
	ASI_MODE_TRIG_SOFT_EDGE ASI_CAMERA_MODE = C.ASI_MODE_TRIG_SOFT_EDGE
	ASI_MODE_TRIG_RISE_EDGE ASI_CAMERA_MODE = C.ASI_MODE_TRIG_RISE_EDGE
	ASI_MODE_TRIG_FALL_EDGE ASI_CAMERA_MODE = C.ASI_MODE_TRIG_FALL_EDGE
	ASI_MODE_TRIG_SOFT_LEVEL ASI_CAMERA_MODE = C.ASI_MODE_TRIG_SOFT_LEVEL
	ASI_MODE_TRIG_HIGH_LEVEL ASI_CAMERA_MODE = C.ASI_MODE_TRIG_HIGH_LEVEL
	ASI_MODE_TRIG_LOW_LEVEL ASI_CAMERA_MODE = C.ASI_MODE_TRIG_LOW_LEVEL
	ASI_MODE_END ASI_CAMERA_MODE = C.ASI_MODE_END
)
func (p ASI_CAMERA_MODE) String() string {
	text := map[ASI_CAMERA_MODE]string{
		ASI_MODE_NORMAL: "ASI_MODE_NORMAL",
		ASI_MODE_TRIG_SOFT_EDGE: "ASI_MODE_TRIG_SOFT_EDGE",
		ASI_MODE_TRIG_RISE_EDGE: "ASI_MODE_TRIG_RISE_EDGE",
		ASI_MODE_TRIG_FALL_EDGE: "ASI_MODE_TRIG_FALL_EDGE",
		ASI_MODE_TRIG_SOFT_LEVEL: "ASI_MODE_TRIG_SOFT_LEVEL",
		ASI_MODE_TRIG_HIGH_LEVEL: "ASI_MODE_TRIG_HIGH_LEVEL",
		ASI_MODE_TRIG_LOW_LEVEL: "ASI_MODE_TRIG_LOW_LEVEL",
		ASI_MODE_END: "ASI_MODE_END",
	}
	return text[p]
}
func (p ASI_CAMERA_MODE) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", p.String())), nil
}

type ASI_TRIG_OUTPUT int
const (
	ASI_TRIG_OUTPUT_PINA ASI_TRIG_OUTPUT = C.ASI_TRIG_OUTPUT_PINA
	ASI_TRIG_OUTPUT_PINB ASI_TRIG_OUTPUT = C.ASI_TRIG_OUTPUT_PINB
	ASI_TRIG_OUTPUT_NONE ASI_TRIG_OUTPUT = C.ASI_TRIG_OUTPUT_NONE
)
func (p ASI_TRIG_OUTPUT) String() string {
	text := map[ASI_TRIG_OUTPUT]string{
		ASI_TRIG_OUTPUT_PINA: "ASI_TRIG_OUTPUT_PINA",
		ASI_TRIG_OUTPUT_PINB: "ASI_TRIG_OUTPUT_PINB",
		ASI_TRIG_OUTPUT_NONE: "ASI_TRIG_OUTPUT_NONE",
	}
	return text[p]
}
func (p ASI_TRIG_OUTPUT) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", p.String())), nil
}

type ASI_ERROR_CODE int
const (
	ASI_SUCCESS ASI_ERROR_CODE = C.ASI_SUCCESS
	ASI_ERROR_INVALID_INDEX ASI_ERROR_CODE = C.ASI_ERROR_INVALID_INDEX
	ASI_ERROR_INVALID_ID ASI_ERROR_CODE = C.ASI_ERROR_INVALID_ID
	ASI_ERROR_INVALID_CONTROL_TYPE ASI_ERROR_CODE = C.ASI_ERROR_INVALID_CONTROL_TYPE
	ASI_ERROR_CAMERA_CLOSED ASI_ERROR_CODE = C.ASI_ERROR_CAMERA_CLOSED
	ASI_ERROR_CAMERA_REMOVED ASI_ERROR_CODE = C.ASI_ERROR_CAMERA_REMOVED
	ASI_ERROR_INVALID_PATH ASI_ERROR_CODE = C.ASI_ERROR_INVALID_PATH
	ASI_ERROR_INVALID_FILEFORMAT ASI_ERROR_CODE = C.ASI_ERROR_INVALID_FILEFORMAT
	ASI_ERROR_INVALID_SIZE ASI_ERROR_CODE = C.ASI_ERROR_INVALID_SIZE
	ASI_ERROR_INVALID_IMGTYPE ASI_ERROR_CODE = C.ASI_ERROR_INVALID_IMGTYPE
	ASI_ERROR_OUTOF_BOUNDARY ASI_ERROR_CODE = C.ASI_ERROR_OUTOF_BOUNDARY
	ASI_ERROR_TIMEOUT ASI_ERROR_CODE = C.ASI_ERROR_TIMEOUT
	ASI_ERROR_INVALID_SEQUENCE ASI_ERROR_CODE = C.ASI_ERROR_INVALID_SEQUENCE
	ASI_ERROR_BUFFER_TOO_SMALL ASI_ERROR_CODE = C.ASI_ERROR_BUFFER_TOO_SMALL
	ASI_ERROR_VIDEO_MODE_ACTIVE ASI_ERROR_CODE = C.ASI_ERROR_VIDEO_MODE_ACTIVE
	ASI_ERROR_EXPOSURE_IN_PROGRESS ASI_ERROR_CODE = C.ASI_ERROR_EXPOSURE_IN_PROGRESS
	ASI_ERROR_GENERAL_ERROR ASI_ERROR_CODE = C.ASI_ERROR_GENERAL_ERROR
	ASI_ERROR_INVALID_MODE ASI_ERROR_CODE = C.ASI_ERROR_INVALID_MODE
	ASI_ERROR_END ASI_ERROR_CODE = C.ASI_ERROR_END
)
func (p ASI_ERROR_CODE) String() string {
	text := map[ASI_ERROR_CODE]string{
		ASI_SUCCESS: "ASI_SUCCESS",
		ASI_ERROR_INVALID_INDEX: "ASI_ERROR_INVALID_INDEX",
		ASI_ERROR_INVALID_ID: "ASI_ERROR_INVALID_ID",
		ASI_ERROR_INVALID_CONTROL_TYPE: "ASI_ERROR_INVALID_CONTROL_TYPE",
		ASI_ERROR_CAMERA_CLOSED: "ASI_ERROR_CAMERA_CLOSED",
		ASI_ERROR_CAMERA_REMOVED: "ASI_ERROR_CAMERA_REMOVED",
		ASI_ERROR_INVALID_PATH: "ASI_ERROR_INVALID_PATH",
		ASI_ERROR_INVALID_FILEFORMAT: "ASI_ERROR_INVALID_FILEFORMAT",
		ASI_ERROR_INVALID_SIZE: "ASI_ERROR_INVALID_SIZE",
		ASI_ERROR_INVALID_IMGTYPE: "ASI_ERROR_INVALID_IMGTYPE",
		ASI_ERROR_OUTOF_BOUNDARY: "ASI_ERROR_OUTOF_BOUNDARY",
		ASI_ERROR_TIMEOUT: "ASI_ERROR_TIMEOUT",
		ASI_ERROR_INVALID_SEQUENCE: "ASI_ERROR_INVALID_SEQUENCE",
		ASI_ERROR_BUFFER_TOO_SMALL: "ASI_ERROR_BUFFER_TOO_SMALL",
		ASI_ERROR_VIDEO_MODE_ACTIVE: "ASI_ERROR_VIDEO_MODE_ACTIVE",
		ASI_ERROR_EXPOSURE_IN_PROGRESS: "ASI_ERROR_EXPOSURE_IN_PROGRESS",
		ASI_ERROR_GENERAL_ERROR: "ASI_ERROR_GENERAL_ERROR",
		ASI_ERROR_INVALID_MODE: "ASI_ERROR_INVALID_MODE",
		ASI_ERROR_END: "ASI_ERROR_END",
	}
	return text[p]
}
func (p ASI_ERROR_CODE) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", p.String())), nil
}
func (p ASI_ERROR_CODE) Error() string {
	return p.String()
}

type ASI_BOOL int
const (
	ASI_FALSE ASI_BOOL = C.ASI_FALSE
	ASI_TRUE  ASI_BOOL = C.ASI_TRUE
)
func (p ASI_BOOL) String() string {
	text := map[ASI_BOOL]string{
		ASI_FALSE: "ASI_FALSE",
		ASI_TRUE: "ASI_TRUE",
	}
	return text[p]
}
func (p ASI_BOOL) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", p.String())), nil
}

// ApiVersion returns the version from SDK
func ApiVersion() (string, error) {
	v, err := C.ASIGetSDKVersion()
	return C.GoString(v), err
}

// ConnectedCameras returns the number of connected cameras
func ConnectedCameras() (int, error) {
	c, err := C.ASIGetNumOfConnectedCameras()
	return int(c), err
}

type ASI_CAMERA_INFO struct {
	Name string //the name of the camera, you can display this to the UI
	CameraID int //this is used to control everything of the camera in other functions.Start from 0.
	MaxHeight int //the max height of the camera
	MaxWidth int //the max width of the camera
	IsColorCam ASI_BOOL
	BayerPattern ASI_BAYER_PATTERN 
	
	SupportedBins [16]int; //1 means bin1 which is supported by every camera, 2 means bin 2 etc.. 0 is the end of supported binning method
	SupportedVideoFormat [8]ASI_IMG_TYPE //this array will content with the support output format type.IMG_END is the end of supported video format
	
	PixelSize float64 //the pixel size of the camera, unit is um. such like 5.6um
	MechanicalShutter ASI_BOOL
	ST4Port ASI_BOOL
	IsCoolerCam ASI_BOOL
	IsUSB3Host ASI_BOOL 
	IsUSB3Camera ASI_BOOL 
	ElecPerADU float64
	BitDepth int 
	IsTriggerCam ASI_BOOL
}

func CameraInfo(index int) (info ASI_CAMERA_INFO, err error) {
	w, err := C.camera_properties(C.int(index))
	if w.retcode != 0 {
		return info, ASI_ERROR_CODE(w.retcode)
	}
	info.Name = C.GoString(&w.info.Name[0])
	info.CameraID = int(w.info.CameraID)
	info.MaxHeight = int(w.info.MaxHeight)
	info.MaxWidth = int(w.info.MaxWidth)
	info.IsColorCam = ASI_BOOL(w.info.IsColorCam)
	info.BayerPattern = ASI_BAYER_PATTERN(w.info.BayerPattern) 
	info.PixelSize = float64(w.info.PixelSize) //the pixel size of the camera, unit is um. such like 5.6um
	info.MechanicalShutter = ASI_BOOL(w.info.MechanicalShutter)
	info.ST4Port = ASI_BOOL(w.info.ST4Port)
	info.IsCoolerCam = ASI_BOOL(w.info.IsCoolerCam)
	info.IsUSB3Host = ASI_BOOL(w.info.IsUSB3Host) 
	info.IsUSB3Camera = ASI_BOOL(w.info.IsUSB3Camera) 
	info.ElecPerADU = float64(w.info.ElecPerADU)
	info.BitDepth = int(w.info.BitDepth) 
	info.IsTriggerCam = ASI_BOOL(w.info.IsTriggerCam)
	for i := 0; i < 16; i++ {
		info.SupportedBins[i] = int(w.info.SupportedBins[i])
	}
	for i := 0; i < 8; i++ {
		info.SupportedVideoFormat[i] = ASI_IMG_TYPE(w.info.SupportedVideoFormat[i])
	}
	C.free_properties(w)
	return info, err
}
