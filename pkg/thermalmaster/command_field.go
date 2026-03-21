package thermalmaster

import "encoding/binary"

// commandWithRegister creates a copy of a base command with a value set in the
// register field (bytes 4-5) and recomputes the CRC.
func commandWithRegister(base [CommandSize]byte, value uint16) [CommandSize]byte {
	cmd := base
	binary.LittleEndian.PutUint16(cmd[4:6], value)
	binary.LittleEndian.PutUint16(cmd[16:18], CRC16CCITT(cmd[:16]))
	return cmd
}

// commandWithByte5 creates a copy of a base command with a single byte value
// set at byte[5] and recomputes the CRC.
// This position is used for palette index and similar commands.
func commandWithByte5(base [CommandSize]byte, value byte) [CommandSize]byte {
	cmd := base
	cmd[5] = value
	binary.LittleEndian.PutUint16(cmd[16:18], CRC16CCITT(cmd[:16]))
	return cmd
}

// commandWithRegisterAndData creates a copy with register (bytes 4-5) and
// a data value in bytes 6-7, then recomputes the CRC.
func commandWithRegisterAndData(
	base [CommandSize]byte,
	register uint16,
	data uint16,
) [CommandSize]byte {
	cmd := base
	binary.LittleEndian.PutUint16(cmd[4:6], register)
	binary.LittleEndian.PutUint16(cmd[6:8], data)
	binary.LittleEndian.PutUint16(cmd[16:18], CRC16CCITT(cmd[:16]))
	return cmd
}
