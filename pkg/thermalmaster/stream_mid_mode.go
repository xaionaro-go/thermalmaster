package thermalmaster

import "fmt"

// StreamMidMode represents a stream mid-processing mode.
type StreamMidMode uint8

const (
	StreamMidPicture StreamMidMode = 0
	StreamMidTPD     StreamMidMode = 1
	StreamMidLCE     StreamMidMode = 2
	StreamMidIR      StreamMidMode = 3
	StreamMidSNR     StreamMidMode = 4
)

// SetStreamMidMode sets the stream mid mode.
func (d *Device) SetStreamMidMode(mode StreamMidMode) error {
	return d.SendCommandNoResponse(commandWithRegister(CmdSetStreamMidMode, uint16(mode)))
}

// GetStreamMidMode reads the current stream mid mode.
func (d *Device) GetStreamMidMode() (StreamMidMode, error) {
	resp, err := d.SendCommandWithResponse(CmdGetStreamMidMode, 1)
	if err != nil {
		return 0, fmt.Errorf("getting stream mid mode: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing stream mid mode response: %w", err)
	}

	return StreamMidMode(v), nil
}
