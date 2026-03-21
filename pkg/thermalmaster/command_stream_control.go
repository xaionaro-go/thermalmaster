package thermalmaster

// Stream control, shutter, and status commands.
var (
	// Stream control commands (cmdType=0x012F).
	CmdStartStream = BuildCommand(0x012F, 0x0081, 0, 1)
	CmdGainLow     = BuildCommand(0x012F, 0x0041, 0, 0)
	CmdGainHigh    = BuildCommand(0x012F, 0x0041, 1, 0)

	// Shutter command (cmdType=0x0136).
	CmdShutter = BuildCommand(0x0136, 0x0043, 0, 0)

	// Status check command (cmdType=0x1021).
	CmdStatus = BuildCommand(0x1021, 0x0081, 0, 2)
)
