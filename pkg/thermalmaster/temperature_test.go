package thermalmaster

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRawToKelvin(t *testing.T) {
	// 19200 / 64 = 300.0 K
	assert.InDelta(t, 300.0, RawThermalValue(19200).Kelvin(), 0.001)
}

func TestRawToCelsius(t *testing.T) {
	// 300.0 K - 273.15 = 26.85 C
	assert.InDelta(t, 26.85, RawThermalValue(19200).Celsius(), 0.001)
}

func TestCelsiusToRaw(t *testing.T) {
	// (26.85 + 273.15) * 64 = 19200
	raw := CelsiusToRaw(26.85)
	assert.InDelta(t, 19200, float64(raw), 1.0)
}

func TestRoundTrip(t *testing.T) {
	// CelsiusToRaw(RawToCelsius(x)) should approximately equal x.
	for _, raw := range []RawThermalValue{15000, 19200, 25000, 30000} {
		celsius := raw.Celsius()
		roundTripped := CelsiusToRaw(celsius)
		assert.InDelta(t, float64(raw), float64(roundTripped), 1.0,
			"round-trip failed for raw=%d", raw)
	}
}

func TestApplyEmissivityCorrectionUnity(t *testing.T) {
	// emissivity=1.0 should return apparentK unchanged.
	apparentK := 300.0
	corrected := ApplyEmissivityCorrection(apparentK, 1.0, 25.0)
	assert.InDelta(t, apparentK, corrected, 0.001)
}

func TestApplyEmissivityCorrectionZero(t *testing.T) {
	// emissivity=0.0 should return apparentK unchanged (guard clause).
	apparentK := 300.0
	corrected := ApplyEmissivityCorrection(apparentK, 0.0, 25.0)
	assert.InDelta(t, apparentK, corrected, 0.001)
}

func TestApplyEmissivityCorrection095(t *testing.T) {
	// With emissivity=0.95 and reflected=25C (298.15K), a 300K apparent
	// reading should correct to a slightly different temperature.
	apparentK := 300.0
	reflectedC := 25.0
	corrected := ApplyEmissivityCorrection(apparentK, 0.95, reflectedC)

	// The corrected value should be close to but not equal to 300K.
	assert.NotEqual(t, apparentK, corrected)

	// Manually verify: T_obj^4 = (T_app^4 - 0.05*T_ref^4) / 0.95
	reflectedK := reflectedC + KelvinOffset
	tApp4 := math.Pow(300.0, 4)
	tRef4 := math.Pow(reflectedK, 4)
	expected := math.Pow((tApp4-0.05*tRef4)/0.95, 0.25)
	assert.InDelta(t, expected, corrected, 0.001)
}

func TestRawToCelsiusCorrected(t *testing.T) {
	env := DefaultEnvParams()
	// With default emissivity=0.95, result should differ slightly from
	// uncorrected Celsius.
	raw := RawThermalValue(19200)
	corrected := raw.CelsiusCorrected(env)
	uncorrected := raw.Celsius()

	// Corrected should be in a reasonable range around the uncorrected value.
	assert.InDelta(t, uncorrected, corrected, 5.0)
}
