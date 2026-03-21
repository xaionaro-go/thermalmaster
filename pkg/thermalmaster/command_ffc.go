package thermalmaster

// FFC/Shutter commands (VDCMD category 0x0210).
var (
	CmdSetAutoFFCStatus        = BuildVDCMD([3]byte{0x41, 0x02, 0x10}, 0, 0) // 0x410210
	CmdSetAutoFFCCurrentParams = BuildVDCMD([3]byte{0x42, 0x02, 0x10}, 0, 0) // 0x420210
	CmdManualFFCUpdate         = BuildVDCMD([3]byte{0x43, 0x02, 0x10}, 0, 0) // 0x430210
	CmdSetShutterManualFFCSwitch = BuildVDCMD([3]byte{0x44, 0x02, 0x10}, 0, 0) // 0x440210
	CmdGetAutoFFCStatus        = BuildVDCMD([3]byte{0x81, 0x02, 0x10}, 1, 1) // 0x810210 (native reads 1 byte)
	CmdGetAutoFFCCurrentParams = BuildVDCMD([3]byte{0x82, 0x02, 0x10}, 1, 1) // 0x820210 (native reads 1 byte)
	CmdGetShutterStatus        = BuildVDCMD([3]byte{0x83, 0x02, 0x10}, 1, 1) // 0x830210 (native reads 1 byte)
)
