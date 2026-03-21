package thermalmaster

import "math"

// Magnus formula constants for saturation vapor pressure calculation.
const (
	magnusA     = 611.2 // Reference pressure (Pa)
	magnusB     = 17.67 // Dimensionless coefficient
	magnusC     = 243.5 // Temperature offset (C)
	vpClampLow  = 800.0 // Minimum physically reasonable vapor pressure (Pa)
	vpClampHigh = 3000.0 // Maximum physically reasonable vapor pressure (Pa)
)

// Atmospheric correction constants.
const (
	maxAtmCorrTempK       = 1273.15 // 1000C in Kelvin; no correction above this
	refAtmCorrTempK       = 410.15  // 137C reference temperature in Kelvin
	lowTempCoef           = 0.01    // Correction coefficient below reference temp
	highTempCoef          = -0.003  // Correction coefficient above reference temp
	atmCorrBaseline       = -0.04   // Baseline correction offset
	refAtmPathLength      = 1425.0  // Atmospheric path length normalization (m)
	log2Reciprocal        = 1.4427  // 1/ln(2)
)

// DistanceTable holds pre-computed distance breakpoints (in meters) used for
// atmospheric transmission lookup. Based on the tables embedded in
// the vendor's temperature compensation library.
var DistanceTable = []float64{
	0.25, 0.5, 0.75, 1.0, 1.25, 1.5, 1.75, 2.0,
	2.5, 3.0, 3.5, 4.0, 4.5, 5.0, 6.0, 7.0,
	8.0, 9.0, 10.0, 12.0, 14.0, 16.0, 18.0, 20.0,
	22.0, 24.0, 26.0, 28.0, 30.0, 32.0, 34.0, 36.0,
	38.0, 40.0, 42.0, 44.0, 46.0, 48.0, 50.0,
}

// CalculateVaporPressure computes the water-vapor partial pressure using the
// Magnus formula. ambientTempC is in Celsius, humidity is a fraction [0,1].
// Returns pressure in Pa, clamped to the range [vpClampLow, vpClampHigh].
func CalculateVaporPressure(ambientTempC, humidity float64) float64 {
	satPressure := magnusA * math.Exp(magnusB*ambientTempC/(ambientTempC+magnusC))
	vp := satPressure * humidity

	// Clamp to physically reasonable range.
	if vp < vpClampLow {
		vp = vpClampLow
	}
	if vp > vpClampHigh {
		vp = vpClampHigh
	}
	return vp
}

// CalculateAtmosphericCoefficient computes the temperature-dependent
// atmospheric correction coefficient. This models how IR radiation interacts
// with the atmosphere differently at different object temperatures.
func CalculateAtmosphericCoefficient(tempC float64) float64 {
	tempK := tempC + KelvinOffset
	if tempK <= 0 {
		// At or below absolute zero: physically impossible, no correction.
		return 0
	}
	if tempK >= maxAtmCorrTempK {
		// Above 1000C: no correction.
		return 0
	}

	if tempK < refAtmCorrTempK {
		ratio := math.Pow(refAtmCorrTempK/tempK, 4)
		return atmCorrBaseline + lowTempCoef*(ratio-1)
	}

	ratio := math.Pow(tempK/refAtmCorrTempK, 4)
	return atmCorrBaseline + highTempCoef*(ratio-1)
}

// RecalculateTau adjusts atmospheric transmission (tau) for the given distance
// and atmospheric coefficient. The coefficient accounts for humidity-dependent
// absorption and temperature-dependent radiation transfer.
func RecalculateTau(
	tau, distance, coef float64,
) float64 {
	distEff := math.Max(distance, 1.0)
	logBase2 := math.Log(distEff) * log2Reciprocal
	return tau * math.Pow(distEff/refAtmPathLength, logBase2*coef)
}
