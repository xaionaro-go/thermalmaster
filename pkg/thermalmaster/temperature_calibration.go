package thermalmaster

import (
	"encoding/binary"
	"math"
)

// commandWithFloat32Data creates a copy of a base command with a float32 value
// encoded as little-endian IEEE 754 in the data field (bytes 6-9), then
// recomputes the CRC.
func commandWithFloat32Data(base [CommandSize]byte, value float32) [CommandSize]byte {
	cmd := base
	binary.LittleEndian.PutUint32(cmd[6:10], math.Float32bits(value))
	binary.LittleEndian.PutUint16(cmd[16:18], CRC16CCITT(cmd[:16]))
	return cmd
}

// RecalTPDBy1Point triggers 1-point temperature recalibration on the device
// using a known reference temperature in Celsius. Unlike RecalTPD1Point (which
// sets a uint16 register value), this encodes the temperature as a float32 in
// the command data field (bytes 6-9).
func (d *Device) RecalTPDBy1Point(tempC float32) error {
	cmd := commandWithFloat32Data(CmdRecalTPD1Point, tempC)
	return d.SendCommandNoResponse(cmd)
}
