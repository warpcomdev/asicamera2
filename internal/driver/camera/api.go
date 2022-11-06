package camera

/*
#cgo CFLAGS:   -I${SRCDIR}/../../../include
#cgo LDFLAGS:  -L${SRCDIR}/../../../lib -l:ASICamera2.lib

#include "ASICamera2.h"
#include "camera.h"
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
var ASI_BAYER_PATTERN_OPTIONS map[ASI_BAYER_PATTERN]string = map[ASI_BAYER_PATTERN]string{
	ASI_BAYER_RG: "ASI_BAYER_RG",
	ASI_BAYER_BG: "ASI_BAYER_BG",
	ASI_BAYER_GR: "ASI_BAYER_GR",
	ASI_BAYER_GB: "ASI_BAYER_GB",
}
func (p ASI_BAYER_PATTERN) String() string {
	return ASI_BAYER_PATTERN_OPTIONS[p]
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
var ASI_IMG_TYPE_OPTIONS map[ASI_IMG_TYPE]string = map[ASI_IMG_TYPE]string{
	ASI_IMG_RAW8: "ASI_IMG_RAW8",
	ASI_IMG_RGB24: "ASI_IMG_RGB24",
	ASI_IMG_RAW16: "ASI_IMG_RAW16",
	ASI_IMG_Y8: "ASI_IMG_Y8",
	ASI_IMG_END: "ASI_IMG_END",
}
func (p ASI_IMG_TYPE) String() string {
	return ASI_IMG_TYPE_OPTIONS[p]
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
var ASI_GUIDE_DIRECTION_OPTIONS map[ASI_GUIDE_DIRECTION]string = map[ASI_GUIDE_DIRECTION]string{
	ASI_GUIDE_NORTH: "ASI_GUIDE_NORTH",
	ASI_GUIDE_SOUTH: "ASI_GUIDE_SOUTH",
	ASI_GUIDE_EAST: "ASI_GUIDE_EAST",
	ASI_GUIDE_WEST: "ASI_GUIDE_WEST",
}
func (p ASI_GUIDE_DIRECTION) String() string {
	return ASI_GUIDE_DIRECTION_OPTIONS[p]
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
var ASI_FLIP_STATUS_OPTIONS map[ASI_FLIP_STATUS]string = map[ASI_FLIP_STATUS]string{
	ASI_FLIP_NONE: "ASI_FLIP_NONE",
	ASI_FLIP_HORIZ: "ASI_FLIP_HORIZ",
	ASI_FLIP_VERT: "ASI_FLIP_VERT",
	ASI_FLIP_BOTH: "ASI_FLIP_BOTH",
}
func (p ASI_FLIP_STATUS) String() string {
	return ASI_FLIP_STATUS_OPTIONS[p]
}
func (p ASI_FLIP_STATUS) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", p.String())), nil
}

type ASI_TRIG_OUTPUT int
const (
	ASI_TRIG_OUTPUT_PINA ASI_TRIG_OUTPUT = C.ASI_TRIG_OUTPUT_PINA
	ASI_TRIG_OUTPUT_PINB ASI_TRIG_OUTPUT = C.ASI_TRIG_OUTPUT_PINB
	ASI_TRIG_OUTPUT_NONE ASI_TRIG_OUTPUT = C.ASI_TRIG_OUTPUT_NONE
)
var ASI_TRIG_OUTPUT_OPTIONS map[ASI_TRIG_OUTPUT]string = map[ASI_TRIG_OUTPUT]string{
	ASI_TRIG_OUTPUT_PINA: "ASI_TRIG_OUTPUT_PINA",
	ASI_TRIG_OUTPUT_PINB: "ASI_TRIG_OUTPUT_PINB",
	ASI_TRIG_OUTPUT_NONE: "ASI_TRIG_OUTPUT_NONE",
}
func (p ASI_TRIG_OUTPUT) String() string {
	return ASI_TRIG_OUTPUT_OPTIONS[p]
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
var ASI_ERROR_CODE_OPTIONS map[ASI_ERROR_CODE]string = map[ASI_ERROR_CODE]string{
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
func (p ASI_ERROR_CODE) String() string {
	return ASI_ERROR_CODE_OPTIONS[p]
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
var ASI_BOOL_OPTIONS map[ASI_BOOL]string = map[ASI_BOOL]string{
	ASI_FALSE: "ASI_FALSE",
	ASI_TRUE: "ASI_TRUE",
}
func (p ASI_BOOL) String() string {
	return ASI_BOOL_OPTIONS[p]
}
func (p ASI_BOOL) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", p.String())), nil
}

// ApiVersion returns the version from SDK
func ASIGetSDKVersion() (string, error) {
	v, err := C.ASIGetSDKVersion()
	return C.GoString(v), err
}

// ConnectedCameras returns the number of connected cameras
func ASIGetNumOfConnectedCameras() (int, error) {
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
	
	SupportedBins []int; //1 means bin1 which is supported by every camera, 2 means bin 2 etc.. 0 is the end of supported binning method
	SupportedVideoFormat []ASI_IMG_TYPE //this array will content with the support output format type.IMG_END is the end of supported video format
	
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

func ASIGetCameraProperty(index int) (info ASI_CAMERA_INFO, err error) {
	w, err := C.wrap_ASIGetCameraProperty(C.int(index))
	if err != nil {
		return info, err
	}
	if w.retcode != 0 {
		return info, ASI_ERROR_CODE(w.retcode)
	}
	info.Name = C.GoString(&(w.info.Name[0]))
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
	info.SupportedBins = make([]int, 0, 16)
	for i := 0; i < 16; i++ {
		bin := int(w.info.SupportedBins[i])
		if bin == 0 {
			break
		}
		info.SupportedBins = append(info.SupportedBins, bin)
	}
	info.SupportedVideoFormat = make([]ASI_IMG_TYPE, 0, 8)
	for i := 0; i < 8; i++ {
		vf := ASI_IMG_TYPE(w.info.SupportedVideoFormat[i])
		if vf == ASI_IMG_END {
			break
		}
		info.SupportedVideoFormat = append(info.SupportedVideoFormat, vf)
	}
	return info, err
}


type ASI_CONTROL_TYPE int
const (
	ASI_GAIN ASI_CONTROL_TYPE = C.ASI_GAIN
	ASI_EXPOSURE ASI_CONTROL_TYPE = C.ASI_EXPOSURE
	ASI_GAMMA ASI_CONTROL_TYPE = C.ASI_GAMMA
	ASI_WB_R ASI_CONTROL_TYPE = C.ASI_WB_R
	ASI_WB_B ASI_CONTROL_TYPE = C.ASI_WB_B
	ASI_OFFSET ASI_CONTROL_TYPE = C.ASI_OFFSET
	ASI_BANDWIDTHOVERLOAD ASI_CONTROL_TYPE = C.ASI_BANDWIDTHOVERLOAD
	ASI_OVERCLOCK ASI_CONTROL_TYPE = C.ASI_OVERCLOCK
	ASI_TEMPERATURE ASI_CONTROL_TYPE = C.ASI_TEMPERATURE
	ASI_FLIP ASI_CONTROL_TYPE = C.ASI_FLIP
	ASI_AUTO_MAX_GAIN ASI_CONTROL_TYPE = C.ASI_AUTO_MAX_GAIN
	ASI_AUTO_MAX_EXP ASI_CONTROL_TYPE = C.ASI_AUTO_MAX_EXP
	ASI_AUTO_TARGET_BRIGHTNESS ASI_CONTROL_TYPE = C.ASI_AUTO_TARGET_BRIGHTNESS
	ASI_HARDWARE_BIN ASI_CONTROL_TYPE = C.ASI_HARDWARE_BIN
	ASI_HIGH_SPEED_MODE ASI_CONTROL_TYPE = C.ASI_HIGH_SPEED_MODE
	ASI_COOLER_POWER_PERC ASI_CONTROL_TYPE = C.ASI_COOLER_POWER_PERC
	ASI_TARGET_TEMP ASI_CONTROL_TYPE = C.ASI_TARGET_TEMP
	ASI_COOLER_ON ASI_CONTROL_TYPE = C.ASI_COOLER_ON
	ASI_MONO_BIN ASI_CONTROL_TYPE = C.ASI_MONO_BIN
	ASI_FAN_ON ASI_CONTROL_TYPE = C.ASI_FAN_ON
	ASI_PATTERN_ADJUST ASI_CONTROL_TYPE = C.ASI_PATTERN_ADJUST
	ASI_ANTI_DEW_HEATER ASI_CONTROL_TYPE = C.ASI_ANTI_DEW_HEATER
)
var ASI_CONTROL_TYPE_OPTIONS map[ASI_CONTROL_TYPE]string = map[ASI_CONTROL_TYPE]string{
	ASI_GAIN: "ASI_GAIN", 
	ASI_EXPOSURE: "ASI_EXPOSURE", 
	ASI_GAMMA: "ASI_GAMMA", 
	ASI_WB_R: "ASI_WB_R", 
	ASI_WB_B: "ASI_WB_B", 
	ASI_OFFSET: "ASI_OFFSET", 
	ASI_BANDWIDTHOVERLOAD: "ASI_BANDWIDTHOVERLOAD", 
	ASI_OVERCLOCK: "ASI_OVERCLOCK", 
	ASI_TEMPERATURE: "ASI_TEMPERATURE", 
	ASI_FLIP: "ASI_FLIP", 
	ASI_AUTO_MAX_GAIN: "ASI_AUTO_MAX_GAIN", 
	ASI_AUTO_MAX_EXP: "ASI_AUTO_MAX_EXP", 
	ASI_AUTO_TARGET_BRIGHTNESS: "ASI_AUTO_TARGET_BRIGHTNESS", 
	ASI_HARDWARE_BIN: "ASI_HARDWARE_BIN", 
	ASI_HIGH_SPEED_MODE: "ASI_HIGH_SPEED_MODE", 
	ASI_COOLER_POWER_PERC: "ASI_COOLER_POWER_PERC", 
	ASI_TARGET_TEMP: "ASI_TARGET_TEMP", 
	ASI_COOLER_ON: "ASI_COOLER_ON", 
	ASI_MONO_BIN: "ASI_MONO_BIN", 
	ASI_FAN_ON: "ASI_FAN_ON", 
	ASI_PATTERN_ADJUST: "ASI_PATTERN_ADJUST", 
	ASI_ANTI_DEW_HEATER: "ASI_ANTI_DEW_HEATER",
}
func (p ASI_CONTROL_TYPE) String() string {
	return ASI_CONTROL_TYPE_OPTIONS[p]
}
func (p ASI_CONTROL_TYPE) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", p.String())), nil
}

type ASI_CONTROL_CAPS struct {
	Name string
	Description string
	MaxValue int
	MinValue int
	DefaultValue int
	IsAutoSupported ASI_BOOL
	IsWritable ASI_BOOL
	ControlType ASI_CONTROL_TYPE
}

type ASI_EXPOSURE_STATUS int
const (
	ASI_EXP_IDLE ASI_EXPOSURE_STATUS = C.ASI_EXP_IDLE
	ASI_EXP_WORKING ASI_EXPOSURE_STATUS = C.ASI_EXP_WORKING
	ASI_EXP_SUCCESS ASI_EXPOSURE_STATUS = C.ASI_EXP_SUCCESS
	ASI_EXP_FAILED ASI_EXPOSURE_STATUS = C.ASI_EXP_FAILED
)
var ASI_EXPOSURE_STATUS_OPTIONS map[ASI_EXPOSURE_STATUS]string = map[ASI_EXPOSURE_STATUS]string{
	ASI_EXP_IDLE: "ASI_EXP_IDLE",
	ASI_EXP_WORKING: "ASI_EXP_WORKING",
	ASI_EXP_SUCCESS: "ASI_EXP_SUCCESS",
	ASI_EXP_FAILED: "ASI_EXP_FAILED",
}
func (p ASI_EXPOSURE_STATUS) String() string {
	return ASI_EXPOSURE_STATUS_OPTIONS[p]
}
func (p ASI_EXPOSURE_STATUS) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", p.String())), nil
}

type ASI_ID struct {
	ID [8]byte
}

func ASIOpenCamera(iCameraID int) error {
	retcode, err := C.ASIOpenCamera(C.int(iCameraID))
	if err != nil {
		return err
	}
	if retcode != 0 {
		return ASI_ERROR_CODE(retcode)
	}
	return nil
}

func ASIInitCamera(iCameraID int) error {
	retcode, err := C.ASIInitCamera(C.int(iCameraID))
	if err != nil {
		return err
	}
	if retcode != 0 {
		return ASI_ERROR_CODE(retcode)
	}
	return nil
}

func ASICloseCamera(iCameraID int) error {
	retcode, err := C.ASICloseCamera(C.int(iCameraID))
	if err != nil {
		return err
	}
	if retcode != 0 {
		return ASI_ERROR_CODE(retcode)
	}
	return nil
}


func ASIGetControlCaps(iCameraID int) ([]ASI_CONTROL_CAPS, error) {
	wrapper, err := C.wrap_ASIGetControlCaps(C.int(iCameraID))
	if err != nil {
		return nil, err
	}
	if wrapper.retcode != 0 {
		return nil, ASI_ERROR_CODE(wrapper.retcode);
	}
	defer C.free_control_wrapper(wrapper);
	controls := make([]ASI_CONTROL_CAPS, 0, int(wrapper.control_num))
	curr := wrapper.alloc
	for i := 0; i < int(wrapper.control_num); i++ {
		control := ASI_CONTROL_CAPS {
			Name: C.GoString(&(curr.info.Name[0])),
			Description: C.GoString(&(curr.info.Description[0])),
			MaxValue: int(curr.info.MaxValue),
			MinValue: int(curr.info.MinValue),
			DefaultValue: int(curr.info.DefaultValue),
			IsAutoSupported: ASI_BOOL(curr.info.IsAutoSupported),
			IsWritable: ASI_BOOL(curr.info.IsWritable),
			ControlType: ASI_CONTROL_TYPE(curr.info.ControlType),
		}
		controls = append(controls, control)
		curr = curr.next
	}
	return controls, nil
}

func ASIGetControlValue(iCameraID int, controlType ASI_CONTROL_TYPE) (plValue int, pbAuto ASI_BOOL, err error) {
	var value C.long
	var auto C.int
	retcode, err := C.ASIGetControlValue(C.int(iCameraID), C.int(controlType), &value, &auto)
	if err != nil {
		return plValue, pbAuto, err
	}
	if retcode != 0 {
		return plValue, pbAuto, ASI_ERROR_CODE(retcode)
	}
	return int(value), ASI_BOOL(auto), nil
}

func ASISetControlValue(iCameraID int, controlType ASI_CONTROL_TYPE, plValue int, pbAuto ASI_BOOL) error {
	retcode, err := C.ASISetControlValue(C.int(iCameraID), C.int(controlType), C.long(plValue), C.int(pbAuto))
	if err != nil {
		return err
	}
	if retcode != 0 {
		return ASI_ERROR_CODE(retcode)
	}
	return nil
}

func ASIGetProductIDs() ([]int, error) {
	wrapper, err := C.wrap_ASIGetProductIDs()
	if err != nil {
		return nil, err
	}
	if wrapper.retcode != 0 {
		return nil, ASI_ERROR_CODE(wrapper.retcode)
	}
	defer C.free_ppid_wrapper(wrapper)
	ppids := make([]int, 0, wrapper.control_num)
	curr := wrapper.alloc
	for i := 0; i < int(wrapper.control_num); i++ {
		ppids = append(ppids, int(curr.info))
		curr = curr.next
	}
	return ppids, nil
}

func ASIGetDroppedFrames(iCameraID int) (int, error) {
	var frames C.int
	retcode, err := C.ASIGetDroppedFrames(C.int(iCameraID), &frames)
	if err != nil {
		return 0, err
	}
	if retcode != 0 {
		return 0, ASI_ERROR_CODE(retcode)
	}
	return int(frames), nil
}

func ASIGetID(iCameraID int) (id [8]byte, err error) {
	var asid C.ASI_ID
	retcode, err := C.ASIGetID(C.int(iCameraID), &asid)
	if err != nil {
		return id, err
	}
	if retcode != 0 {
		return id, ASI_ERROR_CODE(retcode)
	}
	for i := 0; i < 8; i++ {
		id[i] = byte(asid.id[i])
	}
	return id, nil
}

func ASIGetSerialNumber(iCameraID int) (sn [8]byte, err error) {
	var asid C.ASI_SN
	retcode, err := C.ASIGetSerialNumber(C.int(iCameraID), &asid)
	if err != nil {
		return sn, err
	}
	if retcode != 0 {
		return sn, ASI_ERROR_CODE(retcode)
	}
	for i := 0; i < 8; i++ {
		sn[i] = byte(asid.id[i])
	}
	return sn, nil
}

func ASIGetGainOffset(iCameraID int) (pOffset_HighestDR, pOffset_UnityGain, pGain_LowestRN, pOffset_LowestRN int, err error) {
	var p1, p2, p3, p4 C.int
	retcode, err := C.ASIGetGainOffset(C.int(iCameraID), &p1, &p2, &p3, &p4)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if retcode != 0 {
		return 0, 0, 0, 0, ASI_ERROR_CODE(retcode)
	}
	pOffset_HighestDR = int(p1)
	pOffset_UnityGain = int(p2)
	pGain_LowestRN = int(p3)
	pOffset_LowestRN = int(p4)
	return
}
