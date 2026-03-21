package thermalmaster

// AlarmPoint represents a pixel that exceeds a temperature threshold.
type AlarmPoint struct {
	X, Y  int
	TempC float64
}
