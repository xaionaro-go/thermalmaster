package thermalmaster

import "encoding/binary"

// CommandSize is the fixed size of every command sent to the camera.
const CommandSize = 18

// BuildCommand constructs an 18-byte camera command.
//
// Wire format:
//
//	Bytes  0-1:  cmdType (big-endian — verified against Python reference driver)
//	Bytes  2-3:  param (little-endian)
//	Bytes  4-5:  register (little-endian)
//	Bytes  6-11: reserved (zero)
//	Bytes 12-13: respLen (little-endian)
//	Bytes 14-15: reserved (zero)
//	Bytes 16-17: CRC16-CCITT over bytes 0-15
//
// cmdType uses big-endian despite P3_PROTOCOL.md labeling it "(LE)". The
// Python reference stores cmdType 0x012F as wire bytes {0x01, 0x2F}, which
// is big-endian. All other uint16 fields are genuinely little-endian. This
// was confirmed by byte-for-byte comparison against the working Python
// driver's pre-computed command table.
func BuildCommand(
	cmdType uint16,
	param uint16,
	register uint16,
	respLen uint16,
) [CommandSize]byte {
	var cmd [CommandSize]byte
	binary.BigEndian.PutUint16(cmd[0:2], cmdType)
	binary.LittleEndian.PutUint16(cmd[2:4], param)
	binary.LittleEndian.PutUint16(cmd[4:6], register)
	// bytes 6-11: zero (already zero-initialized)
	binary.LittleEndian.PutUint16(cmd[12:14], respLen)
	// bytes 14-15: zero (already zero-initialized)
	binary.LittleEndian.PutUint16(cmd[16:18], CRC16CCITT(cmd[:16]))
	return cmd
}

// BuildVDCMD constructs a command from a 3-byte VDCMD CmdID.
//
// The mapping:
//
//	CmdID[0]   -> param (low byte; high byte is 0x00)
//	CmdID[1:2] -> cmdType (little-endian encoding of the uint16 value)
//
// Example: VDCMD CmdID 0x412F01 = {0x41, 0x2F, 0x01}
//
//	param   = 0x0041
//	cmdType = LE(0x2F, 0x01) = 0x012F -> stored as BE on wire: {0x01, 0x2F}
func BuildVDCMD(
	cmdID [3]byte,
	register uint16,
	respLen uint16,
) [CommandSize]byte {
	param := uint16(cmdID[0])
	cmdType := binary.LittleEndian.Uint16(cmdID[1:3])
	return BuildCommand(cmdType, param, register, respLen)
}
