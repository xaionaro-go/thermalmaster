package thermalmaster

// Image processing commands (VDCMD category 0x0410).
var (
	CmdSetSceneMode        = BuildVDCMD([3]byte{0x42, 0x04, 0x10}, 0, 0) // 0x420410
	CmdSetNoiseReduction   = BuildVDCMD([3]byte{0x44, 0x04, 0x10}, 0, 0) // 0x440410
	CmdSetDetailEnhance    = BuildVDCMD([3]byte{0x45, 0x04, 0x10}, 0, 0) // 0x450410
	CmdSetROILevel         = BuildVDCMD([3]byte{0x46, 0x04, 0x10}, 0, 0) // 0x460410
	CmdSetBrightness       = BuildVDCMD([3]byte{0x47, 0x04, 0x10}, 0, 0) // 0x470410
	CmdSetContrast         = BuildVDCMD([3]byte{0x48, 0x04, 0x10}, 0, 0) // 0x480410
	CmdSetGlobalContrast   = BuildVDCMD([3]byte{0x4A, 0x04, 0x10}, 0, 0) // 0x4A0410
	CmdSetSpaceNoiseReduce = BuildVDCMD([3]byte{0x4B, 0x04, 0x10}, 0, 0) // 0x4B0410
	CmdSetTimeNoiseReduce  = BuildVDCMD([3]byte{0x4C, 0x04, 0x10}, 0, 0) // 0x4C0410
	CmdSetEdgeEnhance      = BuildVDCMD([3]byte{0x4E, 0x04, 0x10}, 0, 0) // 0x4E0410
	CmdSetProfessionMode   = BuildVDCMD([3]byte{0x50, 0x04, 0x10}, 0, 0) // 0x500410
	CmdGetNoiseReduction   = BuildVDCMD([3]byte{0x84, 0x04, 0x10}, 1, 1) // 0x840410 (native reads 1 byte)
	CmdGetDetailEnhance    = BuildVDCMD([3]byte{0x85, 0x04, 0x10}, 1, 1) // 0x850410 (native reads 1 byte)
	CmdGetROILevel         = BuildVDCMD([3]byte{0x86, 0x04, 0x10}, 1, 1) // 0x860410 (native reads 1 byte)
	CmdGetBrightness       = BuildVDCMD([3]byte{0x87, 0x04, 0x10}, 1, 1) // 0x870410 (native reads 1 byte)
	CmdGetContrast         = BuildVDCMD([3]byte{0x88, 0x04, 0x10}, 1, 1) // 0x880410 (native reads 1 byte)
	CmdGetSceneMode        = BuildVDCMD([3]byte{0x89, 0x04, 0x10}, 1, 1) // 0x890410 (native reads 1 byte)
	CmdGetGlobalContrast   = BuildVDCMD([3]byte{0x8A, 0x04, 0x10}, 1, 1) // 0x8A0410 (native reads 1 byte)
	// NOTE: The spec lists CmdGetProfessionMode as 0x8E0410 which collides
	// with CmdGetEdgeEnhance. Following the consistent +0x40 pattern
	// (set 0x50 → get 0x90), we use 0x900410 for CmdGetProfessionMode.
	CmdGetProfessionMode = BuildVDCMD([3]byte{0x90, 0x04, 0x10}, 1, 1) // 0x900410 (native reads 1 byte)
	CmdGetEdgeEnhance    = BuildVDCMD([3]byte{0x8E, 0x04, 0x10}, 1, 1) // 0x8E0410 (native reads 1 byte)
)
