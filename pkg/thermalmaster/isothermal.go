package thermalmaster

import "fmt"

// IsothermalLimit represents an isothermal temperature limit value.
type IsothermalLimit uint16

// SetIsothermalMode sets the isothermal display mode.
func (d *Device) SetIsothermalMode(mode IsothermalMode) error {
	return d.SendCommandNoResponse(commandWithRegister(CmdSetIsothermalMode, uint16(mode)))
}

// GetIsothermalMode reads the current isothermal display mode.
func (d *Device) GetIsothermalMode() (IsothermalMode, error) {
	resp, err := d.SendCommandWithResponse(CmdGetIsothermalMode, 1)
	if err != nil {
		return 0, fmt.Errorf("getting isothermal mode: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing isothermal mode response: %w", err)
	}

	return IsothermalMode(v), nil
}

// SetIsothermalLimit sets the isothermal temperature limit value.
func (d *Device) SetIsothermalLimit(limit IsothermalLimit) error {
	return d.SendCommandNoResponse(commandWithRegister(CmdSetIsothermalLimit, uint16(limit)))
}

// GetIsothermalLimit reads the current isothermal temperature limit.
func (d *Device) GetIsothermalLimit() (IsothermalLimit, error) {
	resp, err := d.SendCommandWithResponse(CmdGetIsothermalLimit, 1)
	if err != nil {
		return 0, fmt.Errorf("getting isothermal limit: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing isothermal limit response: %w", err)
	}

	return IsothermalLimit(v), nil
}

// SetIsothermalEnabled enables or disables isothermal display.
func (d *Device) SetIsothermalEnabled(enabled bool) error {
	v := uint16(0)
	if enabled {
		v = 1
	}
	return d.SendCommandNoResponse(commandWithRegister(CmdSetIsothermalSwitch, v))
}

// GetIsothermalEnabled reads whether isothermal display is enabled.
func (d *Device) GetIsothermalEnabled() (bool, error) {
	resp, err := d.SendCommandWithResponse(CmdGetIsothermalSwitch, 1)
	if err != nil {
		return false, fmt.Errorf("getting isothermal switch: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return false, fmt.Errorf("parsing isothermal switch response: %w", err)
	}

	return v != 0, nil
}
