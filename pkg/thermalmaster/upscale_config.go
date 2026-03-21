package thermalmaster

// UpscaleConfig controls the joint bilateral upsampling parameters.
type UpscaleConfig struct {
	Factor       int              // Upscale factor (default 2)
	SpatialSigma float64          // Spatial gaussian sigma (default 1.5)
	RangeSigma   float64          // Range gaussian sigma for IR guide (default 25.0)
	WindowRadius int              // Half-size of sampling window (default 1 = 3x3)
	NumWorkers   int              // Number of parallel workers (0 = single-threaded, default 0)
}

// DefaultUpscaleConfig returns default upsampling parameters.
func DefaultUpscaleConfig() UpscaleConfig {
	return UpscaleConfig{
		Factor:       2,
		SpatialSigma: 1.5,
		RangeSigma:   25.0,
		WindowRadius: 1,
		NumWorkers:   0,
	}
}
