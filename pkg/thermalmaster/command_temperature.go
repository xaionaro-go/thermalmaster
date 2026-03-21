package thermalmaster

// Temperature detection commands (VDCMD category 0x0203).
var (
	CmdShowFrameTemp    = BuildVDCMD([3]byte{0x51, 0x02, 0x03}, 0, 0) // 0x510203
	CmdShowPointTemp    = BuildVDCMD([3]byte{0x52, 0x02, 0x03}, 0, 0) // 0x520203
	CmdGetFrameTempInfo = BuildVDCMD([3]byte{0x81, 0x02, 0x03}, 0, 4) // 0x810203
	CmdGetPointTempInfo = BuildVDCMD([3]byte{0x82, 0x02, 0x03}, 0, 4) // 0x820203
	CmdGetLineTempInfo  = BuildVDCMD([3]byte{0x83, 0x02, 0x03}, 0, 4) // 0x830203
	CmdGetRectTempInfo  = BuildVDCMD([3]byte{0x84, 0x02, 0x03}, 0, 4) // 0x840203
)
