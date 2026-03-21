package thermalmaster

// StreamSourceMode represents a video stream source mode (ISP pipeline stage).
type StreamSourceMode uint16

const (
	StreamSourceIR      StreamSourceMode = 0
	StreamSourceKBC     StreamSourceMode = 1
	StreamSourceTNR     StreamSourceMode = 2
	StreamSourceHBCDPC  StreamSourceMode = 3
	StreamSourceVBC     StreamSourceMode = 4
	StreamSourceSNR     StreamSourceMode = 5
	StreamSourceDDE     StreamSourceMode = 6
	StreamSourceAGC     StreamSourceMode = 7
	StreamSourceGamma   StreamSourceMode = 8
	StreamSourceTPD     StreamSourceMode = 9
	StreamSourceMirror  StreamSourceMode = 10
	StreamSourcePicture StreamSourceMode = 11
)

// SetStreamSourceMode sets the video stream source mode.
func (d *Device) SetStreamSourceMode(mode StreamSourceMode) error {
	return d.SendCommandNoResponse(commandWithRegister(CmdSetStreamSourceMode, uint16(mode)))
}

// PauseVideoStream pauses the video stream.
func (d *Device) PauseVideoStream() error {
	return d.SendCommandNoResponse(CmdPauseVideoStream)
}
