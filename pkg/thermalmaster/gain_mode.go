package thermalmaster

// GainMode represents sensor gain mode.
//
// Values match the camera's register encoding (confirmed via Python reference
// driver: LOW=0, HIGH=1).
type GainMode int

const (
	GainLow  GainMode = iota // 0 to 550 C, extended range
	GainHigh                 // -20 to 150 C, higher sensitivity
)
