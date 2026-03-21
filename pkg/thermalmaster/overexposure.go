package thermalmaster

import "fmt"

// SetOverexposureProtection enables or disables overexposure protection
// (all FFC status overexposure switch).
func (d *Device) SetOverexposureProtection(enabled bool) error {
	v := uint16(0)
	if enabled {
		v = 1
	}
	return d.SendCommandNoResponse(commandWithRegister(CmdSetAllFFCStatusOverexposure, v))
}

// GetOverexposureProtection reads whether overexposure protection is enabled.
func (d *Device) GetOverexposureProtection() (bool, error) {
	resp, err := d.SendCommandWithResponse(CmdGetAllFFCStatusOverexposure, 1)
	if err != nil {
		return false, fmt.Errorf("getting overexposure protection: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return false, fmt.Errorf("parsing overexposure protection response: %w", err)
	}

	return v != 0, nil
}
