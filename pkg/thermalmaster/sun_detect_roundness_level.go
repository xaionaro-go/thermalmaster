package thermalmaster

import "fmt"

// SunDetectRoundnessLevel represents a sun detection roundness level.
type SunDetectRoundnessLevel uint16

// SetSunDetectRoundnessLevel sets the sun detection roundness level.
func (d *Device) SetSunDetectRoundnessLevel(level SunDetectRoundnessLevel) error {
	return d.SendCommandNoResponse(commandWithRegister(CmdSetSunDetectRoundnessLevel, uint16(level)))
}

// GetSunDetectRoundnessLevel reads the current sun detection roundness level.
func (d *Device) GetSunDetectRoundnessLevel() (SunDetectRoundnessLevel, error) {
	resp, err := d.SendCommandWithResponse(CmdGetSunDetectRoundnessLevel, 1)
	if err != nil {
		return 0, fmt.Errorf("getting sun detect roundness level: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing sun detect roundness level response: %w", err)
	}

	return SunDetectRoundnessLevel(v), nil
}
