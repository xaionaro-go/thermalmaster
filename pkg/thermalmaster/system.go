package thermalmaster

import (
	"encoding/binary"
	"fmt"
)

// deviceTempScale is the divisor for device-internal temperature values
// (reported in units of 0.01 degrees Celsius).
const deviceTempScale = 100.0

// GetDeviceTemp reads the device internal temperature.
// The response is a uint16 (2 bytes, little-endian) representing temperature
// in units of 0.01 degrees Kelvin.
func (d *Device) GetDeviceTemp() (float64, error) {
	resp, err := d.SendCommandWithResponse(CmdGetDeviceTemp, 2)
	if err != nil {
		return 0, fmt.Errorf("getting device temp: %w", err)
	}
	if len(resp) < 2 {
		return 0, fmt.Errorf("device temp response too short: got %d bytes, need 2", len(resp))
	}

	raw := binary.LittleEndian.Uint16(resp[0:2])
	return float64(raw) / deviceTempScale, nil
}

// SaveSystemParams saves current system parameters to non-volatile storage.
func (d *Device) SaveSystemParams() error {
	return d.SendCommandNoResponse(CmdSaveSystemParams)
}

// RestoreSystemParams restores system parameters from non-volatile storage.
func (d *Device) RestoreSystemParams() error {
	return d.SendCommandNoResponse(CmdRestoreSystemParams)
}

// ResetToRom resets the device to ROM defaults.
func (d *Device) ResetToRom() error {
	return d.SendCommandNoResponse(CmdResetToRom)
}

// ResetToBootloader resets the device into bootloader mode.
func (d *Device) ResetToBootloader() error {
	return d.SendCommandNoResponse(CmdResetToBootloader)
}

// EnterRebootMode triggers a device reboot.
func (d *Device) EnterRebootMode() error {
	return d.SendCommandNoResponse(CmdEnterRebootMode)
}

// GetPoweredTime reads the total powered-on time in seconds.
func (d *Device) GetPoweredTime() (uint32, error) {
	resp, err := d.SendCommandWithResponse(CmdGetPoweredTime, 4)
	if err != nil {
		return 0, fmt.Errorf("getting powered time: %w", err)
	}
	if len(resp) < 4 {
		return 0, fmt.Errorf("powered time response too short: got %d bytes, need 4", len(resp))
	}

	return binary.LittleEndian.Uint32(resp[:4]), nil
}

// GetDeviceCurrentStatus reads the device current status register.
func (d *Device) GetDeviceCurrentStatus() (DeviceStatus, error) {
	resp, err := d.SendCommandWithResponse(CmdGetDeviceCurrentStatus, 4)
	if err != nil {
		return 0, fmt.Errorf("getting device status: %w", err)
	}

	v, err := parseUint16Response(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing device status response: %w", err)
	}

	return DeviceStatus(v), nil
}

// GetCRGValue reads the CRG (clock/reset/generation) value from the device.
func (d *Device) GetCRGValue() (CRGValue, error) {
	resp, err := d.SendCommandWithResponse(CmdGetCRGValue, 1)
	if err != nil {
		return 0, fmt.Errorf("getting CRG value: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing CRG value response: %w", err)
	}

	return CRGValue(v), nil
}
