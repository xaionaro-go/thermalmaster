package thermalmaster

import "fmt"

// SetGain sets the sensor gain mode.
//
// GainHigh provides higher sensitivity (-20 to 150 C).
// GainLow provides extended range (0 to 550 C).
func (d *Device) SetGain(mode GainMode) error {
	switch mode {
	case GainHigh:
		return d.SendCommandNoResponse(CmdGainHigh)
	case GainLow:
		return d.SendCommandNoResponse(CmdGainLow)
	default:
		return fmt.Errorf("unknown gain mode: %d", mode)
	}
}

// GetGain reads the current sensor gain mode.
func (d *Device) GetGain() (GainMode, error) {
	resp, err := d.SendCommandWithResponse(CmdGetGainVDCMD, 1)
	if err != nil {
		return 0, fmt.Errorf("getting gain: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing gain response: %w", err)
	}

	return GainMode(v), nil
}
