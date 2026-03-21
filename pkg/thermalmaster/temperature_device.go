package thermalmaster

import (
	"encoding/binary"
	"fmt"
	"math"
)

// commandWithThreeUint16 creates a copy of a base command with three uint16
// values placed at bytes 4-5 (register), 6-7, and 8-9, then recomputes
// the CRC.
func commandWithThreeUint16(
	base [CommandSize]byte,
	a, b, c uint16,
) [CommandSize]byte {
	cmd := base
	binary.LittleEndian.PutUint16(cmd[4:6], a)
	binary.LittleEndian.PutUint16(cmd[6:8], b)
	binary.LittleEndian.PutUint16(cmd[8:10], c)
	binary.LittleEndian.PutUint16(cmd[16:18], CRC16CCITT(cmd[:16]))
	return cmd
}

// SetTPDPointCoord configures the coordinates for a device-side point
// temperature measurement. idx selects the measurement slot (0-based);
// x and y are pixel coordinates in the thermal frame.
func (d *Device) SetTPDPointCoord(
	idx, x, y int,
) error {
	cmd := commandWithThreeUint16(
		CmdShowPointTemp,
		uint16(idx),
		uint16(x),
		uint16(y),
	)
	return d.SendCommandNoResponse(cmd)
}

// GetPointTempInfo retrieves the device-computed temperature for the
// configured measurement points. The response is an array of float32
// values (little-endian IEEE 754).
func (d *Device) GetPointTempInfo() ([]PointTempResult, error) {
	resp, err := d.SendCommandWithResponse(CmdGetPointTempInfo, 16)
	if err != nil {
		return nil, fmt.Errorf("getting point temp info: %w", err)
	}
	if len(resp) < 4 {
		return nil, fmt.Errorf("point temp response too short: %d bytes", len(resp))
	}

	count := len(resp) / 4
	results := make([]PointTempResult, count)
	for i := range results {
		bits := binary.LittleEndian.Uint32(resp[i*4 : i*4+4])
		results[i].TempC = math.Float32frombits(bits)
	}
	return results, nil
}

// GetFrameTempInfo retrieves the device-computed frame temperature info.
// The exact layout depends on the firmware; typically includes min, max,
// and average temperatures as float32 values.
func (d *Device) GetFrameTempInfo() (*FrameTempResult, error) {
	resp, err := d.SendCommandWithResponse(CmdGetFrameTempInfo, 16)
	if err != nil {
		return nil, fmt.Errorf("getting frame temp info: %w", err)
	}
	if len(resp) < 4 {
		return nil, fmt.Errorf("frame temp response too short: %d bytes", len(resp))
	}

	count := len(resp) / 4
	result := &FrameTempResult{
		Values: make([]float32, count),
	}
	for i := range result.Values {
		bits := binary.LittleEndian.Uint32(resp[i*4 : i*4+4])
		result.Values[i] = math.Float32frombits(bits)
	}
	return result, nil
}

// ShowFrameTemp enables device-side frame temperature display with the given
// mode. The mode value is placed in the register field.
func (d *Device) ShowFrameTemp(mode FrameTempDisplayMode) error {
	cmd := commandWithRegister(CmdShowFrameTemp, uint16(mode))
	return d.SendCommandNoResponse(cmd)
}
