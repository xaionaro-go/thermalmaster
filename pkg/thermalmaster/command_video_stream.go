package thermalmaster

// Video/Stream commands (VDCMD category 0x0510).
var (
	CmdPauseVideoStream    = BuildVDCMD([3]byte{0x41, 0x05, 0x10}, 0, 0) // 0x410510
	CmdSetStreamSourceMode = BuildVDCMD([3]byte{0x43, 0x05, 0x10}, 0, 0) // 0x430510
)
