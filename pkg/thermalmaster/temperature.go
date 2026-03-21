package thermalmaster

import "math"

// Kelvin converts a raw sensor value to Kelvin.
func (v RawThermalValue) Kelvin() float64 {
	return float64(v) / TempScale
}

// Celsius converts a raw sensor value to Celsius.
func (v RawThermalValue) Celsius() float64 {
	return v.Kelvin() - KelvinOffset
}

// CelsiusToRaw converts Celsius to raw sensor value.
func CelsiusToRaw(celsius float64) RawThermalValue {
	return RawThermalValue((celsius + KelvinOffset) * TempScale)
}

// ApplyEmissivityCorrection applies Stefan-Boltzmann radiometric correction.
//
// T_object = ((T_apparent^4 - (1-e)*T_reflected^4) / e)^0.25
func ApplyEmissivityCorrection(
	apparentK float64,
	emissivity float64,
	reflectedTempC float64,
) float64 {
	if emissivity >= 1.0 || emissivity <= 0.0 {
		return apparentK
	}

	reflectedK := reflectedTempC + KelvinOffset

	tApp4 := math.Pow(apparentK, 4)
	tRef4 := math.Pow(reflectedK, 4)
	tObj4 := (tApp4 - (1.0-emissivity)*tRef4) / emissivity

	if tObj4 <= 0 {
		// Correction produced an unphysical result (cold object with warm
		// background and low emissivity). Fall back to the apparent temperature
		// rather than returning 0 K (-273.15°C) which would corrupt statistics.
		return apparentK
	}
	return math.Pow(tObj4, 0.25)
}

// CelsiusCorrected converts a raw value to Celsius with environmental
// corrections applied.
func (v RawThermalValue) CelsiusCorrected(env EnvParams) float64 {
	apparentK := v.Kelvin()
	correctedK := ApplyEmissivityCorrection(apparentK, env.Emissivity, env.ReflectedTemp)
	return correctedK - KelvinOffset
}
