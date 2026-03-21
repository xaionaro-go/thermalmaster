package thermalmaster

// Calibration commands (VDCMD category 0x1110).
var (
	CmdKValueCalibration   = BuildVDCMD([3]byte{0x41, 0x11, 0x10}, 0, 0) // 0x411110
	CmdKValueCalibCancel   = BuildVDCMD([3]byte{0x42, 0x11, 0x10}, 0, 0) // 0x421110
	CmdKValueClear         = BuildVDCMD([3]byte{0x44, 0x11, 0x10}, 0, 0) // 0x441110
	CmdSetCursorToDPC      = BuildVDCMD([3]byte{0x52, 0x11, 0x10}, 0, 0) // 0x521110
	CmdDPCCalibCancel      = BuildVDCMD([3]byte{0x53, 0x11, 0x10}, 0, 0) // 0x531110
	CmdDPCCalibClear       = BuildVDCMD([3]byte{0x55, 0x11, 0x10}, 0, 0) // 0x551110
	CmdCursorSwitchSet     = BuildVDCMD([3]byte{0x57, 0x11, 0x10}, 0, 0) // 0x571110
	CmdCursorPositionSet   = BuildVDCMD([3]byte{0x58, 0x11, 0x10}, 0, 0) // 0x581110
	CmdRecalTPD1Point      = BuildVDCMD([3]byte{0x71, 0x11, 0x10}, 0, 0) // 0x711110
	CmdRecalTPD1PointCancel = BuildVDCMD([3]byte{0x72, 0x11, 0x10}, 0, 0) // 0x721110
	CmdRecalTPD1PointClear = BuildVDCMD([3]byte{0x74, 0x11, 0x10}, 0, 0) // 0x741110
	CmdRecalTPD2PointCancel = BuildVDCMD([3]byte{0x77, 0x11, 0x10}, 0, 0) // 0x771110
	CmdRecalTPD2PointClear = BuildVDCMD([3]byte{0x79, 0x11, 0x10}, 0, 0) // 0x791110
	CmdGetCursorSwitch     = BuildVDCMD([3]byte{0x81, 0x11, 0x10}, 1, 1) // 0x811110 (native reads 1 byte)
	CmdGetCursorPosition   = BuildVDCMD([3]byte{0x82, 0x11, 0x10}, 0, 4) // 0x821110 (2 uint16 coordinates = 4 bytes)
)
