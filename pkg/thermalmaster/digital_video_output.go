package thermalmaster

import "fmt"

// DigitalVideoOutput represents a digital video output mode.
type DigitalVideoOutput uint8

const (
	DVOLowGain30HzTemp  DigitalVideoOutput = 0
	DVOHighGain30HzTemp DigitalVideoOutput = 1
	DVOHighGain30HzImg  DigitalVideoOutput = 2
	DVOHighGain60HzImg  DigitalVideoOutput = 3
)

// SetDigitalVideoOutput sets the digital video output mode.
func (d *Device) SetDigitalVideoOutput(mode DigitalVideoOutput) error {
	return d.SendCommandNoResponse(commandWithRegister(CmdSetDigitalVideoOutput, uint16(mode)))
}

// GetDigitalVideoOutput reads the current digital video output mode.
func (d *Device) GetDigitalVideoOutput() (DigitalVideoOutput, error) {
	resp, err := d.SendCommandWithResponse(CmdGetDigitalVideoOutput, 1)
	if err != nil {
		return 0, fmt.Errorf("getting digital video output: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing digital video output response: %w", err)
	}

	return DigitalVideoOutput(v), nil
}
