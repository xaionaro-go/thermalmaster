package thermalmaster

// Environment correction commands (VDCMD category 0x0710).
var (
	CmdSetEnvTU            = BuildVDCMD([3]byte{0x42, 0x07, 0x10}, 0, 0) // 0x420710
	CmdSetEnvTA            = BuildVDCMD([3]byte{0x43, 0x07, 0x10}, 0, 0) // 0x430710
	CmdSetEnvEMS           = BuildVDCMD([3]byte{0x44, 0x07, 0x10}, 0, 0) // 0x440710
	CmdSetEnvTAU           = BuildVDCMD([3]byte{0x45, 0x07, 0x10}, 0, 0) // 0x450710
	CmdSetEnvCorrectSwitch = BuildVDCMD([3]byte{0x46, 0x07, 0x10}, 0, 0) // 0x460710
	CmdGetEnvTU            = BuildVDCMD([3]byte{0x82, 0x07, 0x10}, 1, 1) // 0x820710 (native reads 1 byte)
	CmdGetEnvTA            = BuildVDCMD([3]byte{0x83, 0x07, 0x10}, 1, 1) // 0x830710 (native reads 1 byte)
	CmdGetEnvEMS           = BuildVDCMD([3]byte{0x84, 0x07, 0x10}, 1, 1) // 0x840710 (native reads 1 byte)
	CmdGetEnvTAU           = BuildVDCMD([3]byte{0x85, 0x07, 0x10}, 1, 1) // 0x850710 (native reads 1 byte)
	CmdGetEnvCorrectSwitch = BuildVDCMD([3]byte{0x86, 0x07, 0x10}, 1, 1) // 0x860710 (native reads 1 byte)
)
