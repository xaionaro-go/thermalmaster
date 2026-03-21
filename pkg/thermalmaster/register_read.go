package thermalmaster

import (
	"bytes"
	"fmt"
)

// ReadRegister sends a register read command and returns the decoded string
// response with trailing null bytes removed.
func (d *Device) ReadRegister(
	cmd [CommandSize]byte,
	length int,
) (string, error) {
	resp, err := d.SendCommandWithResponse(cmd, length)
	if err != nil {
		return "", err
	}
	resp = bytes.TrimRight(resp, "\x00")
	return string(resp), nil
}

// ReadDeviceInfo reads all device identification registers.
func (d *Device) ReadDeviceInfo() (DeviceInfo, error) {
	var info DeviceInfo
	var err error

	info.Model, err = d.ReadRegister(CmdReadName, 30)
	if err != nil {
		return info, fmt.Errorf("reading model: %w", err)
	}

	info.FWVersion, err = d.ReadRegister(CmdReadVersion, 12)
	if err != nil {
		return info, fmt.Errorf("reading firmware version: %w", err)
	}

	info.PartNumber, err = d.ReadRegister(CmdReadPartNumber, 64)
	if err != nil {
		return info, fmt.Errorf("reading part number: %w", err)
	}

	info.Serial, err = d.ReadRegister(CmdReadSerial, 64)
	if err != nil {
		return info, fmt.Errorf("reading serial: %w", err)
	}

	info.HWVersion, err = d.ReadRegister(CmdReadHWVersion, 64)
	if err != nil {
		return info, fmt.Errorf("reading HW version: %w", err)
	}

	info.ModelLong, err = d.ReadRegister(CmdReadModelLong, 64)
	if err != nil {
		return info, fmt.Errorf("reading model (long): %w", err)
	}

	return info, nil
}
