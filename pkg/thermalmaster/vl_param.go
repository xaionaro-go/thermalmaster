package thermalmaster

import "fmt"

// WriteVLParam writes a raw VL (ISP) parameter value at the given register address.
func (d *Device) WriteVLParam(register, value uint16) error {
	cmd := commandWithRegisterAndData(CmdWriteVLParam, register, value)
	return d.SendCommandNoResponse(cmd)
}

// ReadVLParam reads a raw VL (ISP) parameter value from the given register address.
func (d *Device) ReadVLParam(register uint16) (uint8, error) {
	cmd := commandWithRegister(CmdReadVLParam, register)
	resp, err := d.SendCommandWithResponse(cmd, 1)
	if err != nil {
		return 0, fmt.Errorf("reading VL param at register 0x%04x: %w", register, err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing VL param response: %w", err)
	}

	return v, nil
}
