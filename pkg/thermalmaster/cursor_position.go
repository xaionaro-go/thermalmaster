package thermalmaster

import (
	"encoding/binary"
	"fmt"
)

// CursorPosition holds X,Y coordinates for the calibration cursor.
type CursorPosition struct {
	X uint16
	Y uint16
}

// SetCursorEnabled enables or disables the calibration cursor.
func (d *Device) SetCursorEnabled(enabled bool) error {
	v := uint16(0)
	if enabled {
		v = 1
	}
	return d.SendCommandNoResponse(commandWithRegister(CmdCursorSwitchSet, v))
}

// GetCursorEnabled reads whether the calibration cursor is enabled.
func (d *Device) GetCursorEnabled() (bool, error) {
	resp, err := d.SendCommandWithResponse(CmdGetCursorSwitch, 1)
	if err != nil {
		return false, fmt.Errorf("getting cursor switch: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return false, fmt.Errorf("parsing cursor switch response: %w", err)
	}

	return v != 0, nil
}

// SetCursorPosition sets the calibration cursor position.
func (d *Device) SetCursorPosition(pos CursorPosition) error {
	cmd := commandWithRegisterAndData(CmdCursorPositionSet, pos.X, pos.Y)
	return d.SendCommandNoResponse(cmd)
}

// GetCursorPosition reads the current calibration cursor position.
func (d *Device) GetCursorPosition() (CursorPosition, error) {
	resp, err := d.SendCommandWithResponse(CmdGetCursorPosition, 4)
	if err != nil {
		return CursorPosition{}, fmt.Errorf("getting cursor position: %w", err)
	}
	if len(resp) < 4 {
		return CursorPosition{}, fmt.Errorf("cursor position response too short: got %d bytes, need 4", len(resp))
	}

	return CursorPosition{
		X: binary.LittleEndian.Uint16(resp[0:2]),
		Y: binary.LittleEndian.Uint16(resp[2:4]),
	}, nil
}
