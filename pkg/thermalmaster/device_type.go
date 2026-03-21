package thermalmaster

import (
	"strings"
)

// DeviceType represents the camera device type as classified by the native
// AC020 SDK's get_current_device_type() function. The type determines which
// VDCMD commands the firmware supports.
type DeviceType uint8

const (
	DeviceTypeUnrecognized DeviceType = 0  // Device name not recognized
	DeviceTypeCS640      DeviceType = 1  // MINI640S
	DeviceTypeG1280S     DeviceType = 2  // G128*S, GL1280S
	DeviceTypeWN2256     DeviceType = 3  // Camera MINI2384, MINI2384*, Camera W*, Camera X*
	DeviceTypeWN2384     DeviceType = 4  // Camera MINI2384 JPEG, Camera W* WN2384, Camera X3
	DeviceTypeWN2640     DeviceType = 5  // Camera MINI2640, Camera W* WN2640, Camera W* N2640 BULK
	DeviceTypeWN2256T    DeviceType = 6  // WN2256T
	DeviceTypeWN2320T    DeviceType = 7  // WN2320T
	DeviceTypeWN2384T    DeviceType = 8  // MINI2 384, WN2384T
	DeviceTypeTIF        DeviceType = 9  // TIF devices
	DeviceTypeGL1280S    DeviceType = 10 // GL1280S*
	DeviceTypeWN2Lite640 DeviceType = 11 // WN2-lite640
	DeviceTypeAC02       DeviceType = 12 // AC0264*
	DeviceTypeP2L        DeviceType = 13 // P2L, P2L-D, P4, AC002
	DeviceTypeTC2C       DeviceType = 14 // TINY-2Y-C, TC2-C
	DeviceTypeP3         DeviceType = 15 // P3, P1, Omni One, TC001 Max, AC001
)

// DeviceTypeFromName returns the device type for a given device name string.
// The device type determines which VDCMD commands are supported.
//
// Device name → type mapping:
//
//	Type 1  (CS640):       "MINI6400S-262-12A300009D01X"
//	Type 2  (G1280S):      "G128*S", "GL1280S"
//	Type 3  (WN2256):      "Camera MINI2256", "Camera W WN2256", "Camera X2",
//	                        "Camera X2L", "Camera X15", "Camera X 919"
//	Type 4  (WN2384):      "Camera MINI2384", "MINI2384 JPEG 84",
//	                        "Camera W WN2384", "Camera X3"
//	Type 5  (WN2640):      "Camera MINI2640", "Camera W WN2640",
//	                        "Camera W N2640 BULK"
//	Type 6  (WN2256T):     "WN2256T"
//	Type 7  (WN2320T):     "WN2320T"
//	Type 8  (WN2384T):     "MINI2 384", "WN2384T"
//	Type 9  (TIF):         TIF-series devices
//	Type 10 (GL1280S):     "GL1280S*"
//	Type 11 (WN2Lite640):  "WN2-lite640"
//	Type 12 (AC02):        "AC0264*"
//	Type 13 (P2L):         "P2L", "P2L-D", "P4", "AC002"
//	Type 14 (TC2C):        "TINY-2Y-C", "TC2-C"
//	Type 15 (P3):          "P3", "P1", "Omni One", "TC001 Max", "AC001"
func DeviceTypeFromName(name string) DeviceType {
	name = strings.TrimRight(name, "\x00")

	switch {
	// Type 1: CS640.
	case strings.HasPrefix(name, "MINI6400S"):
		return DeviceTypeCS640

	// Type 2: G1280S.
	case strings.HasPrefix(name, "G128") && strings.HasSuffix(name, "S"),
		name == "GL1280S":
		return DeviceTypeG1280S

	// Type 10: GL1280S (distinct from type 2).
	case strings.HasPrefix(name, "GL1280S"):
		return DeviceTypeGL1280S

	// Type 4: WN2384 (check before WN2256 since some names overlap).
	case name == "Camera MINI2384",
		strings.HasPrefix(name, "MINI2384"),
		name == "Camera W WN2384",
		name == "Camera X3":
		return DeviceTypeWN2384

	// Type 8: WN2384T.
	case strings.HasPrefix(name, "MINI2 384"),
		name == "WN2384T":
		return DeviceTypeWN2384T

	// Type 3: WN2256.
	case name == "Camera MINI2256",
		strings.HasPrefix(name, "Camera W WN2256"),
		strings.HasPrefix(name, "Camera X2"),
		name == "Camera X15",
		name == "Camera X 919":
		return DeviceTypeWN2256

	// Type 6: WN2256T.
	case name == "WN2256T":
		return DeviceTypeWN2256T

	// Type 5: WN2640.
	case name == "Camera MINI2640",
		strings.HasPrefix(name, "Camera W WN2640"),
		strings.HasPrefix(name, "Camera W N2640 BULK"):
		return DeviceTypeWN2640

	// Type 7: WN2320T.
	case name == "WN2320T":
		return DeviceTypeWN2320T

	// Type 9: TIF.
	case strings.HasPrefix(name, "TIF"):
		return DeviceTypeTIF

	// Type 11: WN2-lite640.
	case strings.HasPrefix(name, "WN2-lite640"):
		return DeviceTypeWN2Lite640

	// Type 12: AC02.
	case strings.HasPrefix(name, "AC0264"):
		return DeviceTypeAC02

	// Type 13: P2L.
	case strings.HasPrefix(name, "P2L"),
		name == "P4",
		name == "AC002":
		return DeviceTypeP2L

	// Type 14: TC2-C.
	case strings.HasPrefix(name, "TC2-C"),
		strings.HasPrefix(name, "TINY-2Y-C"):
		return DeviceTypeTC2C

	// Type 15: P3/P1/Omni One.
	case name == "P3",
		name == "P1",
		name == "Omni One",
		strings.HasPrefix(name, "TC001 Ma"),
		name == "AC001":
		return DeviceTypeP3

	default:
		return DeviceTypeUnrecognized
	}
}
