package thermalmaster

import "fmt"

// ShutterManualFFCSwitch represents the shutter manual FFC switch value.
type ShutterManualFFCSwitch uint16

// TriggerShutter triggers a manual shutter operation (flat-field correction).
func (d *Device) TriggerShutter() error {
	return d.SendCommandNoResponse(CmdShutter)
}

// SetAutoFFCEnabled enables or disables automatic flat-field correction.
func (d *Device) SetAutoFFCEnabled(enabled bool) error {
	v := uint16(0)
	if enabled {
		v = 1
	}
	return d.SendCommandNoResponse(commandWithRegister(CmdSetAutoFFCStatus, v))
}

// GetAutoFFCEnabled reads whether automatic flat-field correction is enabled.
func (d *Device) GetAutoFFCEnabled() (bool, error) {
	resp, err := d.SendCommandWithResponse(CmdGetAutoFFCStatus, 1)
	if err != nil {
		return false, fmt.Errorf("getting auto FFC status: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return false, fmt.Errorf("parsing auto FFC response: %w", err)
	}

	return v != 0, nil
}

// ManualFFCUpdate triggers a manual flat-field correction update.
func (d *Device) ManualFFCUpdate() error {
	return d.SendCommandNoResponse(CmdManualFFCUpdate)
}

// ManualFFCWithGain triggers a manual FFC with gain switching.
func (d *Device) ManualFFCWithGain() error {
	return d.SendCommandNoResponse(CmdManualFFCWithGain)
}

// SetShutterManualFFCSwitch sets the shutter manual FFC switch value.
func (d *Device) SetShutterManualFFCSwitch(value ShutterManualFFCSwitch) error {
	return d.SendCommandNoResponse(commandWithRegister(CmdSetShutterManualFFCSwitch, uint16(value)))
}

// GetShutterStatus reads the current shutter status.
func (d *Device) GetShutterStatus() (ShutterStatus, error) {
	resp, err := d.SendCommandWithResponse(CmdGetShutterStatus, 1)
	if err != nil {
		return 0, fmt.Errorf("getting shutter status: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing shutter status response: %w", err)
	}

	return ShutterStatus(v), nil
}

// SetAutoFFCCurrentParams sets the auto FFC current parameters value.
func (d *Device) SetAutoFFCCurrentParams(value AutoFFCParams) error {
	return d.SendCommandNoResponse(commandWithRegister(CmdSetAutoFFCCurrentParams, uint16(value)))
}

// GetAutoFFCCurrentParams reads the auto FFC current parameters value.
func (d *Device) GetAutoFFCCurrentParams() (AutoFFCParams, error) {
	resp, err := d.SendCommandWithResponse(CmdGetAutoFFCCurrentParams, 1)
	if err != nil {
		return 0, fmt.Errorf("getting auto FFC params: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing auto FFC params response: %w", err)
	}

	return AutoFFCParams(v), nil
}
