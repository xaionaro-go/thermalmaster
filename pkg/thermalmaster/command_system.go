package thermalmaster

// System commands (VDCMD category 0x0101).
var (
	CmdGetDeviceInfo          = BuildVDCMD([3]byte{0x81, 0x01, 0x01}, 0, 4) // 0x810101
	CmdGetDeviceCurrentStatus = BuildVDCMD([3]byte{0x82, 0x01, 0x01}, 0, 4) // 0x820101
	CmdResetToRom             = BuildVDCMD([3]byte{0x42, 0x01, 0x01}, 0, 0) // 0x420101
	CmdResetToBootloader      = BuildVDCMD([3]byte{0x47, 0x01, 0x01}, 0, 0) // 0x470101
	CmdEnterRebootMode        = BuildVDCMD([3]byte{0x4A, 0x01, 0x01}, 0, 0) // 0x4A0101
)
