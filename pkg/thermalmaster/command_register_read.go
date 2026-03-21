package thermalmaster

// Register read commands (cmdType=0x0101, param=0x0081).
var (
	CmdReadName      = BuildCommand(0x0101, 0x0081, 0x01, 30)
	CmdReadVersion   = BuildCommand(0x0101, 0x0081, 0x02, 12)
	CmdReadPartNumber = BuildCommand(0x0101, 0x0081, 0x06, 64)
	CmdReadSerial    = BuildCommand(0x0101, 0x0081, 0x07, 64)
	CmdReadHWVersion = BuildCommand(0x0101, 0x0081, 0x0A, 64)
	CmdReadModelLong = BuildCommand(0x0101, 0x0081, 0x0F, 64)
)
