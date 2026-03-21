package thermalmaster

// Gain direct commands.
var (
	CmdSetGainVDCMD      = BuildVDCMD([3]byte{0x41, 0x2F, 0x01}, 0, 0) // gain set
	CmdGetGainVDCMD      = BuildVDCMD([3]byte{0x81, 0x2F, 0x01}, 1, 1) // gain get (native reads 1 byte)
	CmdManualFFCWithGain = BuildVDCMD([3]byte{0x43, 0x36, 0x01}, 0, 0) // manual FFC with gain
)
