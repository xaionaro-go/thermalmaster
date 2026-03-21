package thermalmaster

// ModelConfig holds model-specific parameters.
type ModelConfig struct {
	Model   Model
	PID     ProductID
	SensorW int // columns
	SensorH int // rows (same for IR and thermal halves)
}

// FrameRows returns the total number of rows in a frame (2 sensor halves + 2 separator rows).
func (c ModelConfig) FrameRows() int { return 2*c.SensorH + 2 }

// FrameSize returns the frame payload size in bytes (16-bit pixels).
func (c ModelConfig) FrameSize() int { return 2 * c.FrameRows() * c.SensorW }

// FrameReadSize returns the total USB read size including start and end markers.
func (c ModelConfig) FrameReadSize() int { return c.FrameSize() + 2*MarkerSize }

// IRRowEnd returns the exclusive end row index of the IR half.
func (c ModelConfig) IRRowEnd() int { return c.SensorH }

// ThermalRowStart returns the start row index of the thermal half (after 2 separator rows).
func (c ModelConfig) ThermalRowStart() int { return c.SensorH + 2 }

// ThermalRowEnd returns the exclusive end row index of the thermal half.
func (c ModelConfig) ThermalRowEnd() int { return 2*c.SensorH + 2 }

var (
	ConfigP3 = ModelConfig{Model: ModelP3, PID: 0x45A2, SensorW: 256, SensorH: 192}
	ConfigP1 = ModelConfig{Model: ModelP1, PID: 0x45C2, SensorW: 160, SensorH: 120}
)
