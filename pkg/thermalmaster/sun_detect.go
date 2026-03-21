package thermalmaster

import "fmt"

// SetSunDetectEnabled enables or disables sun detection.
func (d *Device) SetSunDetectEnabled(enabled bool) error {
	v := uint16(0)
	if enabled {
		v = 1
	}
	return d.SendCommandNoResponse(commandWithRegister(CmdSetSunDetectSwitch, v))
}

// GetSunDetectEnabled reads whether sun detection is enabled.
func (d *Device) GetSunDetectEnabled() (bool, error) {
	resp, err := d.SendCommandWithResponse(CmdGetSunDetectSwitch, 1)
	if err != nil {
		return false, fmt.Errorf("getting sun detect switch: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return false, fmt.Errorf("parsing sun detect switch response: %w", err)
	}

	return v != 0, nil
}
