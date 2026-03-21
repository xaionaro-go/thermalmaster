package thermalmaster

// EnvParams holds environmental parameters for temperature correction.
type EnvParams struct {
	Emissivity    float64 // Surface emissivity (0.01-1.0)
	AmbientTemp   float64 // Ambient temperature (C)
	ReflectedTemp float64 // Reflected/background temperature (C)
	Distance      float64 // Distance to target (meters, 0.25-49.99)
	Humidity      float64 // Relative humidity (0.0-1.0)
}

// DefaultEnvParams returns reasonable default environmental parameters.
func DefaultEnvParams() EnvParams {
	return EnvParams{
		Emissivity:    0.95,
		AmbientTemp:   25.0,
		ReflectedTemp: 25.0,
		Distance:      1.0,
		Humidity:      0.5,
	}
}
