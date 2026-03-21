package thermalmaster

// Device control commands (VDCMD category 0x1010).
var (
	CmdSetMirrorFlip          = BuildVDCMD([3]byte{0x43, 0x10, 0x10}, 0, 0) // 0x431010
	CmdSetStreamMidMode       = BuildVDCMD([3]byte{0x45, 0x10, 0x10}, 0, 0) // 0x451010
	CmdSetDigitalVideoOutput  = BuildVDCMD([3]byte{0x46, 0x10, 0x10}, 0, 0) // 0x461010
	CmdSaveSystemParams       = BuildVDCMD([3]byte{0x51, 0x10, 0x10}, 0, 0) // 0x511010
	CmdRestoreSystemParams    = BuildVDCMD([3]byte{0x52, 0x10, 0x10}, 0, 0) // 0x521010
	CmdHeartbeatStart         = BuildVDCMD([3]byte{0x53, 0x10, 0x10}, 0, 0) // 0x531010
	CmdHeartbeatSend          = BuildVDCMD([3]byte{0x54, 0x10, 0x10}, 0, 0) // 0x541010
	CmdWriteVLParam           = BuildVDCMD([3]byte{0x5C, 0x10, 0x10}, 0, 0) // 0x5C1010
	CmdGetMirrorFlip          = BuildVDCMD([3]byte{0x83, 0x10, 0x10}, 1, 1) // 0x831010 (native reads 1 byte)
	CmdGetStreamMidMode       = BuildVDCMD([3]byte{0x85, 0x10, 0x10}, 1, 1) // 0x851010 (native reads 1 byte)
	CmdGetDigitalVideoOutput  = BuildVDCMD([3]byte{0x86, 0x10, 0x10}, 1, 1) // 0x861010 (native reads 1 byte)
	CmdGetDeviceTemp          = BuildVDCMD([3]byte{0x91, 0x10, 0x10}, 0, 2) // 0x911010 (native reads 1 ushort = 2 bytes)
	CmdGetPoweredTime         = BuildVDCMD([3]byte{0x93, 0x10, 0x10}, 0, 4) // 0x931010 (uint32 = 4 bytes)
	CmdReadVLParam            = BuildVDCMD([3]byte{0x9C, 0x10, 0x10}, 1, 1) // 0x9C1010 (native reads 1 byte)
	CmdGetCRGValue            = BuildVDCMD([3]byte{0xA6, 0x10, 0x10}, 1, 1) // 0xA61010 (native reads 1 byte)
)
