package thermalmaster

import (
	"fmt"
	"strings"
)

// SensorSource selects which sensor data to extract from a raw camera frame.
type SensorSource int

const (
	// SensorThermal uses 16-bit raw thermal data.
	SensorThermal SensorSource = iota
	// SensorIR uses 8-bit IR brightness data (hardware AGC'd).
	SensorIR
	// SensorBlended uses joint-bilateral-upscaled thermal data guided by IR edges.
	SensorBlended
)

// ParseSensorSource parses a sensor source name (case-insensitive).
func ParseSensorSource(s string) (SensorSource, error) {
	switch strings.ToLower(s) {
	case "thermal":
		return SensorThermal, nil
	case "ir":
		return SensorIR, nil
	case "blended":
		return SensorBlended, nil
	default:
		return 0, fmt.Errorf("unknown sensor: %q (use: thermal, ir, blended)", s)
	}
}
