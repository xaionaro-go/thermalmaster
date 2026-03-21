package thermalmaster

// Palette/Isothermal commands (VDCMD category 0x0310).
var (
	CmdSetPaletteIdx              = BuildVDCMD([3]byte{0x45, 0x03, 0x10}, 0, 0) // 0x450310
	CmdSetIsothermalMode          = BuildVDCMD([3]byte{0x46, 0x03, 0x10}, 0, 0) // 0x460310
	CmdSetIsothermalLimit         = BuildVDCMD([3]byte{0x47, 0x03, 0x10}, 0, 0) // 0x470310
	CmdSetIsothermalSwitch        = BuildVDCMD([3]byte{0x48, 0x03, 0x10}, 0, 0) // 0x480310
	CmdSetSunDetectPixelRatio     = BuildVDCMD([3]byte{0x49, 0x03, 0x10}, 0, 0) // 0x490310
	CmdSetSunDetectRoundnessLevel = BuildVDCMD([3]byte{0x4A, 0x03, 0x10}, 0, 0) // 0x4A0310
	CmdSetSunDetectSwitch         = BuildVDCMD([3]byte{0x4B, 0x03, 0x10}, 0, 0) // 0x4B0310
	CmdSetAllFFCStatusOverexposure = BuildVDCMD([3]byte{0x4C, 0x03, 0x10}, 0, 0) // 0x4C0310
	CmdGetPaletteIdx              = BuildVDCMD([3]byte{0x85, 0x03, 0x10}, 1, 1) // 0x850310 (native reads 1 byte)
	CmdGetIsothermalMode          = BuildVDCMD([3]byte{0x86, 0x03, 0x10}, 1, 1) // 0x860310 (native reads 1 byte)
	CmdGetIsothermalLimit         = BuildVDCMD([3]byte{0x87, 0x03, 0x10}, 1, 1) // 0x870310 (native reads 1 byte)
	CmdGetIsothermalSwitch        = BuildVDCMD([3]byte{0x88, 0x03, 0x10}, 1, 1) // 0x880310 (native reads 1 byte)
	CmdGetSunDetectPixelRatio     = BuildVDCMD([3]byte{0x89, 0x03, 0x10}, 1, 1) // 0x890310 (native reads 1 byte)
	CmdGetSunDetectRoundnessLevel = BuildVDCMD([3]byte{0x8A, 0x03, 0x10}, 1, 1) // 0x8A0310 (native reads 1 byte)
	CmdGetSunDetectSwitch         = BuildVDCMD([3]byte{0x8B, 0x03, 0x10}, 1, 1) // 0x8B0310 (native reads 1 byte)
	CmdGetAllFFCStatusOverexposure = BuildVDCMD([3]byte{0x8D, 0x03, 0x10}, 1, 1) // 0x8D0310 (native reads 1 byte)
)
