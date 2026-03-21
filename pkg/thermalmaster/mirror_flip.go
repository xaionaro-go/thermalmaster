package thermalmaster

import "fmt"

// MirrorFlipMode represents the image mirror/flip orientation.
type MirrorFlipMode uint8

const (
	MirrorFlipNone MirrorFlipMode = 0
	MirrorOnly     MirrorFlipMode = 1
	FlipOnly       MirrorFlipMode = 2
	MirrorAndFlip  MirrorFlipMode = 3
)

// SetMirrorFlip sets the image mirror/flip mode.
func (d *Device) SetMirrorFlip(mode MirrorFlipMode) error {
	return d.setRegister(CmdSetMirrorFlip, uint16(mode))
}

// GetMirrorFlip reads the current image mirror/flip mode.
func (d *Device) GetMirrorFlip() (MirrorFlipMode, error) {
	resp, err := d.SendCommandWithResponse(CmdGetMirrorFlip, 1)
	if err != nil {
		return 0, fmt.Errorf("getting mirror/flip: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing mirror/flip response: %w", err)
	}

	return MirrorFlipMode(v), nil
}
