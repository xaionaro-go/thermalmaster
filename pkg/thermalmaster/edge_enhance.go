package thermalmaster

import "fmt"

// SetEdgeEnhance sets the edge enhancement level.
func (d *Device) SetEdgeEnhance(level EdgeEnhanceLevel) error {
	return d.setRegister(CmdSetEdgeEnhance, uint16(level))
}

// GetEdgeEnhance reads the current edge enhancement level.
//
// NOTE: The protocol spec maps both CmdGetEdgeEnhance and CmdGetProfessionMode
// to VDCMD 0x8E0410. This may return the profession mode value instead if the
// spec has a collision. See command_image_processing.go for details.
func (d *Device) GetEdgeEnhance() (EdgeEnhanceLevel, error) {
	resp, err := d.SendCommandWithResponse(CmdGetEdgeEnhance, 1)
	if err != nil {
		return 0, fmt.Errorf("getting edge enhance: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing edge enhance response: %w", err)
	}

	return EdgeEnhanceLevel(v), nil
}
