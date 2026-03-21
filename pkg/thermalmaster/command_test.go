package thermalmaster

import (
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCommand(t *testing.T) {
	t.Run("read_name", func(t *testing.T) {
		cmd := BuildCommand(0x0101, 0x0081, 0x0001, 30)

		// cmdType is big-endian on the wire.
		assert.Equal(t, uint16(0x0101), binary.BigEndian.Uint16(cmd[0:2]), "cmdType")
		// All other fields are little-endian.
		assert.Equal(t, uint16(0x0081), binary.LittleEndian.Uint16(cmd[2:4]), "param")
		assert.Equal(t, uint16(0x0001), binary.LittleEndian.Uint16(cmd[4:6]), "register")
		assert.Equal(t, uint16(30), binary.LittleEndian.Uint16(cmd[12:14]), "respLen")

		// Reserved bytes 6-11 and 14-15 must be zero.
		for i := 6; i < 12; i++ {
			assert.Equal(t, byte(0), cmd[i], "reserved byte %d", i)
		}
		assert.Equal(t, byte(0), cmd[14], "reserved byte 14")
		assert.Equal(t, byte(0), cmd[15], "reserved byte 15")

		// CRC over first 16 bytes must match known value.
		expectedCRC := CRC16CCITT(cmd[:16])
		actualCRC := binary.LittleEndian.Uint16(cmd[16:18])
		assert.Equal(t, expectedCRC, actualCRC, "CRC")
		assert.Equal(t, uint16(0x904F), actualCRC, "CRC known value")
	})

	t.Run("command_size", func(t *testing.T) {
		cmd := BuildCommand(0, 0, 0, 0)
		assert.Equal(t, 18, len(cmd))
	})
}

func TestBuildVDCMD(t *testing.T) {
	t.Run("gain_low_set", func(t *testing.T) {
		// VDCMD CmdID 0x412F01: CmdID[0]=0x41 -> param=0x0041,
		// CmdID[1:2] LE = {0x2F, 0x01} -> cmdType=0x012F
		cmd := BuildVDCMD([3]byte{0x41, 0x2F, 0x01}, 0, 0)
		assert.Equal(t, uint16(0x012F), binary.BigEndian.Uint16(cmd[0:2]), "cmdType")
		assert.Equal(t, uint16(0x0041), binary.LittleEndian.Uint16(cmd[2:4]), "param")
		assert.Equal(t, uint16(0x0000), binary.LittleEndian.Uint16(cmd[4:6]), "register")
	})

	t.Run("gain_read", func(t *testing.T) {
		// VDCMD CmdID {0x81, 0x2F, 0x01}: param=0x0081, cmdType=0x012F
		cmd := BuildVDCMD([3]byte{0x81, 0x2F, 0x01}, 0, 0)
		assert.Equal(t, uint16(0x012F), binary.BigEndian.Uint16(cmd[0:2]), "cmdType")
		assert.Equal(t, uint16(0x0081), binary.LittleEndian.Uint16(cmd[2:4]), "param")
	})

	t.Run("env_correct_switch", func(t *testing.T) {
		// VDCMD CmdID 0x460710: CmdID bytes {0x46, 0x07, 0x10}
		// param = 0x0046, cmdType = LE(0x07, 0x10) = 0x1007
		cmd := BuildVDCMD([3]byte{0x46, 0x07, 0x10}, 0, 0)
		assert.Equal(t, uint16(0x1007), binary.BigEndian.Uint16(cmd[0:2]), "cmdType")
		assert.Equal(t, uint16(0x0046), binary.LittleEndian.Uint16(cmd[2:4]), "param")
	})

	t.Run("crc_is_valid", func(t *testing.T) {
		cmd := BuildVDCMD([3]byte{0x41, 0x2F, 0x01}, 0, 0)
		expectedCRC := CRC16CCITT(cmd[:16])
		actualCRC := binary.LittleEndian.Uint16(cmd[16:18])
		assert.Equal(t, expectedCRC, actualCRC, "CRC must be consistent")
	})
}

func TestPrebuiltCommands(t *testing.T) {
	t.Run("CmdReadName", func(t *testing.T) {
		assert.Equal(t, uint16(0x0101), binary.BigEndian.Uint16(CmdReadName[0:2]))
		assert.Equal(t, uint16(0x0081), binary.LittleEndian.Uint16(CmdReadName[2:4]))
		assert.Equal(t, uint16(0x0001), binary.LittleEndian.Uint16(CmdReadName[4:6]))
		assert.Equal(t, uint16(30), binary.LittleEndian.Uint16(CmdReadName[12:14]))
	})

	t.Run("CmdStartStream", func(t *testing.T) {
		assert.Equal(t, uint16(0x012F), binary.BigEndian.Uint16(CmdStartStream[0:2]))
		assert.Equal(t, uint16(0x0081), binary.LittleEndian.Uint16(CmdStartStream[2:4]))
		assert.Equal(t, uint16(0x0000), binary.LittleEndian.Uint16(CmdStartStream[4:6]))
		assert.Equal(t, uint16(1), binary.LittleEndian.Uint16(CmdStartStream[12:14]))
	})

	t.Run("CmdGainLow", func(t *testing.T) {
		assert.Equal(t, uint16(0x012F), binary.BigEndian.Uint16(CmdGainLow[0:2]))
		assert.Equal(t, uint16(0x0041), binary.LittleEndian.Uint16(CmdGainLow[2:4]))
		assert.Equal(t, uint16(0x0000), binary.LittleEndian.Uint16(CmdGainLow[4:6]))
	})

	t.Run("CmdGainHigh", func(t *testing.T) {
		assert.Equal(t, uint16(0x012F), binary.BigEndian.Uint16(CmdGainHigh[0:2]))
		assert.Equal(t, uint16(0x0041), binary.LittleEndian.Uint16(CmdGainHigh[2:4]))
		assert.Equal(t, uint16(0x0001), binary.LittleEndian.Uint16(CmdGainHigh[4:6]))
	})

	t.Run("CmdShutter", func(t *testing.T) {
		assert.Equal(t, uint16(0x0136), binary.BigEndian.Uint16(CmdShutter[0:2]))
		assert.Equal(t, uint16(0x0043), binary.LittleEndian.Uint16(CmdShutter[2:4]))
	})

	t.Run("CmdStatus", func(t *testing.T) {
		assert.Equal(t, uint16(0x1021), binary.BigEndian.Uint16(CmdStatus[0:2]))
		assert.Equal(t, uint16(0x0081), binary.LittleEndian.Uint16(CmdStatus[2:4]))
		assert.Equal(t, uint16(2), binary.LittleEndian.Uint16(CmdStatus[12:14]))
	})

	t.Run("all_commands_have_valid_crc", func(t *testing.T) {
		commands := map[string][18]byte{
			"CmdReadName":       CmdReadName,
			"CmdReadVersion":    CmdReadVersion,
			"CmdStartStream":    CmdStartStream,
			"CmdGainLow":        CmdGainLow,
			"CmdGainHigh":       CmdGainHigh,
			"CmdShutter":        CmdShutter,
			"CmdStatus":         CmdStatus,
			"CmdSetGainVDCMD":   CmdSetGainVDCMD,
			"CmdGetGainVDCMD":   CmdGetGainVDCMD,
			"CmdHeartbeatStart": CmdHeartbeatStart,
			"CmdHeartbeatSend":  CmdHeartbeatSend,
		}
		for name, cmd := range commands {
			expectedCRC := CRC16CCITT(cmd[:16])
			actualCRC := binary.LittleEndian.Uint16(cmd[16:18])
			assert.Equal(t, expectedCRC, actualCRC, "CRC mismatch for %s", name)
		}
	})

	t.Run("all_commands_are_18_bytes", func(t *testing.T) {
		// Compile-time guarantee via [18]byte type, but verify at test level too.
		require.Equal(t, 18, len(CmdReadName))
		require.Equal(t, 18, len(CmdStartStream))
	})
}

// TestCommandsMatchPythonReference verifies that all pre-computed commands
// produce identical wire bytes to the Python reference driver (p3_camera.py).
// This is the authoritative test: if our bytes match the Python driver's
// known-working command table, the commands are correct.
func TestCommandsMatchPythonReference(t *testing.T) {
	pythonCommands := map[string]string{
		"read_name":    "0101810001000000000000001e0000004f90",
		"read_version": "0101810002000000000000000c0000001f63",
		"start_stream": "012f81000000000000000000010000004930",
		"gain_low":     "012f41000000000000000000000000003c3a",
		"gain_high":    "012f41000100000000000000000000004939",
		"shutter":      "01364300000000000000000000000000cd0b",
		"status":       "1021810000000000000000000200000095d1",
	}

	goCommands := map[string][18]byte{
		"read_name":    CmdReadName,
		"read_version": CmdReadVersion,
		"start_stream": CmdStartStream,
		"gain_low":     CmdGainLow,
		"gain_high":    CmdGainHigh,
		"shutter":      CmdShutter,
		"status":       CmdStatus,
	}

	for name, pyHex := range pythonCommands {
		t.Run(name, func(t *testing.T) {
			goCmd, ok := goCommands[name]
			require.True(t, ok, "missing Go command for %s", name)
			goHex := hex.EncodeToString(goCmd[:])
			assert.Equal(t, pyHex, goHex,
				"Go command bytes do not match Python reference for %s", name)
		})
	}
}
