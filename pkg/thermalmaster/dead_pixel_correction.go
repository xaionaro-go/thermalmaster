package thermalmaster

// DPCCursorValue represents a cursor value for dead pixel correction.
type DPCCursorValue uint16

// SetCursorToDPC sets the cursor position for dead pixel correction.
func (d *Device) SetCursorToDPC(value DPCCursorValue) error {
	return d.SendCommandNoResponse(commandWithRegister(CmdSetCursorToDPC, uint16(value)))
}

// DPCCalibCancel cancels an in-progress dead pixel correction calibration.
func (d *Device) DPCCalibCancel() error {
	return d.SendCommandNoResponse(CmdDPCCalibCancel)
}

// DPCCalibClear clears the dead pixel correction data.
func (d *Device) DPCCalibClear() error {
	return d.SendCommandNoResponse(CmdDPCCalibClear)
}
