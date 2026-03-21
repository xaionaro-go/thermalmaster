package thermalmaster

// setRegister sends a value via the register field (bytes [4:6]).
func (d *Device) setRegister(cmd [CommandSize]byte, value uint16) error {
	return d.SendCommandNoResponse(commandWithRegister(cmd, value))
}
