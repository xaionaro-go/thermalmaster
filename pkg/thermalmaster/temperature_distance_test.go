package thermalmaster

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateVaporPressure_StandardConditions(t *testing.T) {
	// At 25C and 50% RH, saturation pressure ≈ 3169 Pa, so VP ≈ 1584 Pa.
	// Magnus formula: 611.2 * exp(17.67 * 25 / (25 + 243.5)) = ~3169 Pa.
	vp := CalculateVaporPressure(25.0, 0.5)
	expected := 611.2 * math.Exp(17.67*25.0/(25.0+243.5)) * 0.5
	assert.InDelta(t, expected, vp, 1.0)
}

func TestCalculateVaporPressure_ClampLow(t *testing.T) {
	// Very cold and dry: should clamp to 800.
	vp := CalculateVaporPressure(-20.0, 0.1)
	assert.Equal(t, 800.0, vp)
}

func TestCalculateVaporPressure_ClampHigh(t *testing.T) {
	// Very hot and humid: should clamp to 3000.
	vp := CalculateVaporPressure(40.0, 1.0)
	assert.Equal(t, 3000.0, vp)
}

func TestCalculateVaporPressure_ZeroHumidity(t *testing.T) {
	// Zero humidity: VP = 0, should clamp to 800.
	vp := CalculateVaporPressure(25.0, 0.0)
	assert.Equal(t, 800.0, vp)
}

func TestCalculateAtmosphericCoefficient_RoomTemp(t *testing.T) {
	// At 25C (298.15K) < refTempK (410.15K), coefficient should be computed
	// from the low-temperature branch.
	coef := CalculateAtmosphericCoefficient(25.0)
	tempK := 25.0 + KelvinOffset
	refTempK := 410.15
	ratio := math.Pow(refTempK/tempK, 4)
	expected := -0.04 + 0.01*(ratio-1)
	assert.InDelta(t, expected, coef, 1e-6)
}

func TestCalculateAtmosphericCoefficient_HighTemp(t *testing.T) {
	// At 200C (473.15K) > refTempK (410.15K), coefficient should be computed
	// from the high-temperature branch.
	coef := CalculateAtmosphericCoefficient(200.0)
	tempK := 200.0 + KelvinOffset
	refTempK := 410.15
	ratio := math.Pow(tempK/refTempK, 4)
	expected := -0.04 + (-0.003)*(ratio-1)
	assert.InDelta(t, expected, coef, 1e-6)
}

func TestCalculateAtmosphericCoefficient_BelowZeroCelsius(t *testing.T) {
	// -10C = 263.15K: valid temperature, should produce a correction.
	// 263.15K < refAtmCorrTempK (410.15K), so uses low-temp branch.
	coef := CalculateAtmosphericCoefficient(-10.0)
	assert.NotEqual(t, 0.0, coef, "sub-zero Celsius should still produce correction")

	// Below absolute zero: guard clause returns 0.
	coef = CalculateAtmosphericCoefficient(-300.0)
	assert.Equal(t, 0.0, coef)
}

func TestCalculateAtmosphericCoefficient_Above1000C(t *testing.T) {
	coef := CalculateAtmosphericCoefficient(1000.0)
	assert.Equal(t, 0.0, coef)
}

func TestRecalculateTau_DistanceOne(t *testing.T) {
	// At distance=1.0, distEff=1.0, log(1)=0 so logBase2=0,
	// result = tau * (1/1425)^0 = tau * 1 = tau.
	result := RecalculateTau(0.9, 1.0, -0.03)
	assert.InDelta(t, 0.9, result, 1e-9)
}

func TestRecalculateTau_DistanceLessThanOne(t *testing.T) {
	// Distance < 1.0 is clamped to 1.0, so same result as distance=1.0.
	result := RecalculateTau(0.9, 0.5, -0.03)
	assert.InDelta(t, 0.9, result, 1e-9)
}

func TestRecalculateTau_LargerDistance(t *testing.T) {
	tau := 0.9
	distance := 5.0
	coef := -0.03
	result := RecalculateTau(tau, distance, coef)

	// Manual computation.
	logBase2 := math.Log(5.0) * 1.4427
	expected := tau * math.Pow(5.0/1425.0, logBase2*coef)
	assert.InDelta(t, expected, result, 1e-9)
}

func TestRecalculateTau_ZeroCoef(t *testing.T) {
	// With coef=0, the exponent is 0 so result = tau * 1 = tau.
	result := RecalculateTau(0.85, 10.0, 0.0)
	assert.InDelta(t, 0.85, result, 1e-9)
}
