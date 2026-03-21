package thermalmaster

import "fmt"

// SunDetectPixelRatio represents a sun detection pixel ratio threshold.
type SunDetectPixelRatio uint16

// SetSunDetectPixelRatio sets the sun detection pixel ratio threshold.
func (d *Device) SetSunDetectPixelRatio(ratio SunDetectPixelRatio) error {
	return d.SendCommandNoResponse(commandWithRegister(CmdSetSunDetectPixelRatio, uint16(ratio)))
}

// GetSunDetectPixelRatio reads the current sun detection pixel ratio threshold.
func (d *Device) GetSunDetectPixelRatio() (SunDetectPixelRatio, error) {
	resp, err := d.SendCommandWithResponse(CmdGetSunDetectPixelRatio, 1)
	if err != nil {
		return 0, fmt.Errorf("getting sun detect pixel ratio: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing sun detect pixel ratio response: %w", err)
	}

	return SunDetectPixelRatio(v), nil
}
