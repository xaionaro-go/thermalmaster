package thermalmaster

// TempInfo holds temperature statistics for a region.
type TempInfo struct {
	Min  float64
	Max  float64
	Avg  float64
	MinX int
	MinY int
	MaxX int
	MaxY int
}
