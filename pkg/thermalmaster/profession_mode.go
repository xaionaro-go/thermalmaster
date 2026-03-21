package thermalmaster

import "fmt"

// ProfessionMode represents a profession/application mode.
type ProfessionMode uint8

const (
	ProfessionNormal       ProfessionMode = 0
	ProfessionProfessional ProfessionMode = 1
)

// SetProfessionMode sets the profession mode.
func (d *Device) SetProfessionMode(mode ProfessionMode) error {
	return d.setRegister(CmdSetProfessionMode, uint16(mode))
}

// GetProfessionMode reads the current profession mode.
//
// NOTE: The protocol spec maps CmdGetProfessionMode to VDCMD 0x8E0410, which
// collides with CmdGetEdgeEnhance. The actual behavior depends on the firmware.
// See command_image_processing.go for details.
func (d *Device) GetProfessionMode() (ProfessionMode, error) {
	resp, err := d.SendCommandWithResponse(CmdGetProfessionMode, 1)
	if err != nil {
		return 0, fmt.Errorf("getting profession mode: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing profession mode response: %w", err)
	}

	return ProfessionMode(v), nil
}
