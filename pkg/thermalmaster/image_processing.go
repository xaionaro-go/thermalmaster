package thermalmaster

import "fmt"

// SetBrightness sets the image brightness level.
func (d *Device) SetBrightness(level BrightnessLevel) error {
	return d.setRegister(CmdSetBrightness, uint16(level))
}

// GetBrightness reads the current image brightness level.
func (d *Device) GetBrightness() (BrightnessLevel, error) {
	resp, err := d.SendCommandWithResponse(CmdGetBrightness, 1)
	if err != nil {
		return 0, fmt.Errorf("getting brightness: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing brightness response: %w", err)
	}

	return BrightnessLevel(v), nil
}

// SetContrast sets the image contrast level.
func (d *Device) SetContrast(level ContrastLevel) error {
	return d.setRegister(CmdSetContrast, uint16(level))
}

// GetContrast reads the current image contrast level.
func (d *Device) GetContrast() (ContrastLevel, error) {
	resp, err := d.SendCommandWithResponse(CmdGetContrast, 1)
	if err != nil {
		return 0, fmt.Errorf("getting contrast: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing contrast response: %w", err)
	}

	return ContrastLevel(v), nil
}

// SetDetailEnhance sets the detail enhancement level.
func (d *Device) SetDetailEnhance(level DetailEnhanceLevel) error {
	return d.setRegister(CmdSetDetailEnhance, uint16(level))
}

// GetDetailEnhance reads the current detail enhancement level.
func (d *Device) GetDetailEnhance() (DetailEnhanceLevel, error) {
	resp, err := d.SendCommandWithResponse(CmdGetDetailEnhance, 1)
	if err != nil {
		return 0, fmt.Errorf("getting detail enhance: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing detail enhance response: %w", err)
	}

	return DetailEnhanceLevel(v), nil
}

// SetNoiseReduction sets the noise reduction level.
func (d *Device) SetNoiseReduction(level NoiseReductionLevel) error {
	return d.setRegister(CmdSetNoiseReduction, uint16(level))
}

// GetNoiseReduction reads the current noise reduction level.
func (d *Device) GetNoiseReduction() (NoiseReductionLevel, error) {
	resp, err := d.SendCommandWithResponse(CmdGetNoiseReduction, 1)
	if err != nil {
		return 0, fmt.Errorf("getting noise reduction: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing noise reduction response: %w", err)
	}

	return NoiseReductionLevel(v), nil
}

// SetSpaceNoiseReduce sets the spatial noise reduction level.
// No corresponding get command exists, so no verification is performed.
func (d *Device) SetSpaceNoiseReduce(level NoiseReductionLevel) error {
	return d.setRegister(CmdSetSpaceNoiseReduce, uint16(level))
}

// SetTimeNoiseReduce sets the temporal noise reduction level.
// No corresponding get command exists, so no verification is performed.
func (d *Device) SetTimeNoiseReduce(level NoiseReductionLevel) error {
	return d.setRegister(CmdSetTimeNoiseReduce, uint16(level))
}

// SetGlobalContrast sets the global contrast level.
func (d *Device) SetGlobalContrast(level ContrastLevel) error {
	return d.setRegister(CmdSetGlobalContrast, uint16(level))
}

// GetGlobalContrast reads the current global contrast level.
func (d *Device) GetGlobalContrast() (ContrastLevel, error) {
	resp, err := d.SendCommandWithResponse(CmdGetGlobalContrast, 1)
	if err != nil {
		return 0, fmt.Errorf("getting global contrast: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing global contrast response: %w", err)
	}

	return ContrastLevel(v), nil
}

// SetROILevel sets the region-of-interest AGC level.
func (d *Device) SetROILevel(level ROILevel) error {
	return d.setRegister(CmdSetROILevel, uint16(level))
}

// GetROILevel reads the current region-of-interest AGC level.
func (d *Device) GetROILevel() (ROILevel, error) {
	resp, err := d.SendCommandWithResponse(CmdGetROILevel, 1)
	if err != nil {
		return 0, fmt.Errorf("getting ROI level: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing ROI level response: %w", err)
	}

	return ROILevel(v), nil
}

// SetSceneMode sets the image processing scene mode.
// Only supported on unrecognized devices (device type 0). Recognized devices
// like the P3 do not support this command.
func (d *Device) SetSceneMode(mode SceneMode) error {
	if d.deviceType != DeviceTypeUnrecognized {
		return ErrUnsupportedCommand{Command: "SetSceneMode", DeviceType: d.deviceType}
	}
	return d.setRegister(CmdSetSceneMode, uint16(mode))
}

// GetSceneMode reads the current image processing scene mode.
// See SetSceneMode for device type restrictions.
func (d *Device) GetSceneMode() (SceneMode, error) {
	if d.deviceType != DeviceTypeUnrecognized {
		return 0, ErrUnsupportedCommand{Command: "GetSceneMode", DeviceType: d.deviceType}
	}
	resp, err := d.SendCommandWithResponse(CmdGetSceneMode, 1)
	if err != nil {
		return 0, fmt.Errorf("getting scene mode: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing scene mode response: %w", err)
	}

	return SceneMode(v), nil
}
