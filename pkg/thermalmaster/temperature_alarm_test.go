package thermalmaster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPointOverThresholdAlarm_NoAlarms(t *testing.T) {
	width, height := 8, 8
	baseRaw := CelsiusToRaw(20.0) // ~20C everywhere
	thermal := make([]RawThermalValue, width*height)
	for i := range thermal {
		thermal[i] = baseRaw
	}

	env := EnvParams{Emissivity: 1.0, ReflectedTemp: 25.0}
	points := PointOverThresholdAlarm(thermal, width, height, 30.0, env)
	assert.Empty(t, points)
}

func TestPointOverThresholdAlarm_AllAbove(t *testing.T) {
	width, height := 4, 4
	baseRaw := CelsiusToRaw(50.0) // ~50C everywhere
	thermal := make([]RawThermalValue, width*height)
	for i := range thermal {
		thermal[i] = baseRaw
	}

	env := EnvParams{Emissivity: 1.0, ReflectedTemp: 25.0}
	points := PointOverThresholdAlarm(thermal, width, height, 30.0, env)
	assert.Len(t, points, width*height)
}

func TestPointOverThresholdAlarm_SingleHotPixel(t *testing.T) {
	width, height := 8, 8
	baseRaw := CelsiusToRaw(20.0)
	hotRaw := CelsiusToRaw(50.0)
	thermal := make([]RawThermalValue, width*height)
	for i := range thermal {
		thermal[i] = baseRaw
	}
	// Place a single hot pixel at (3, 5).
	thermal[5*width+3] = hotRaw

	env := EnvParams{Emissivity: 1.0, ReflectedTemp: 25.0}
	points := PointOverThresholdAlarm(thermal, width, height, 30.0, env)
	require.Len(t, points, 1)
	assert.Equal(t, 3, points[0].X)
	assert.Equal(t, 5, points[0].Y)
	assert.InDelta(t, 50.0, points[0].TempC, 0.5)
}

func TestPointOverThresholdAlarm_CorrectCoordinates(t *testing.T) {
	width, height := 4, 4
	baseRaw := CelsiusToRaw(20.0)
	hotRaw := CelsiusToRaw(40.0)
	thermal := make([]RawThermalValue, width*height)
	for i := range thermal {
		thermal[i] = baseRaw
	}
	// Hot pixels at (0,0) and (3,3).
	thermal[0] = hotRaw
	thermal[3*width+3] = hotRaw

	env := EnvParams{Emissivity: 1.0, ReflectedTemp: 25.0}
	points := PointOverThresholdAlarm(thermal, width, height, 30.0, env)
	require.Len(t, points, 2)
	assert.Equal(t, 0, points[0].X)
	assert.Equal(t, 0, points[0].Y)
	assert.Equal(t, 3, points[1].X)
	assert.Equal(t, 3, points[1].Y)
}

func TestRectOverThresholdAlarm_NoAlarm(t *testing.T) {
	width, height := 8, 8
	baseRaw := CelsiusToRaw(20.0)
	thermal := make([]RawThermalValue, width*height)
	for i := range thermal {
		thermal[i] = baseRaw
	}

	env := EnvParams{Emissivity: 1.0, ReflectedTemp: 25.0}
	result := RectOverThresholdAlarm(thermal, 0, 0, 4, 4, width, 30.0, env)
	assert.False(t, result)
}

func TestRectOverThresholdAlarm_HotPixelInside(t *testing.T) {
	width, height := 8, 8
	baseRaw := CelsiusToRaw(20.0)
	hotRaw := CelsiusToRaw(50.0)
	thermal := make([]RawThermalValue, width*height)
	for i := range thermal {
		thermal[i] = baseRaw
	}
	// Hot pixel at (2, 2) which is inside rect (0, 0, 4, 4).
	thermal[2*width+2] = hotRaw

	env := EnvParams{Emissivity: 1.0, ReflectedTemp: 25.0}
	result := RectOverThresholdAlarm(thermal, 0, 0, 4, 4, width, 30.0, env)
	assert.True(t, result)
}

func TestRectOverThresholdAlarm_HotPixelOutside(t *testing.T) {
	width, height := 8, 8
	baseRaw := CelsiusToRaw(20.0)
	hotRaw := CelsiusToRaw(50.0)
	thermal := make([]RawThermalValue, width*height)
	for i := range thermal {
		thermal[i] = baseRaw
	}
	// Hot pixel at (6, 6) which is outside rect (0, 0, 4, 4).
	thermal[6*width+6] = hotRaw

	env := EnvParams{Emissivity: 1.0, ReflectedTemp: 25.0}
	result := RectOverThresholdAlarm(thermal, 0, 0, 4, 4, width, 30.0, env)
	assert.False(t, result)
}

func TestRectOverThresholdAlarm_EdgeOfRect(t *testing.T) {
	width, height := 8, 8
	baseRaw := CelsiusToRaw(20.0)
	hotRaw := CelsiusToRaw(50.0)
	thermal := make([]RawThermalValue, width*height)
	for i := range thermal {
		thermal[i] = baseRaw
	}
	// Hot pixel at (3, 3): last pixel inside rect (0, 0, 4, 4).
	thermal[3*width+3] = hotRaw

	env := EnvParams{Emissivity: 1.0, ReflectedTemp: 25.0}
	result := RectOverThresholdAlarm(thermal, 0, 0, 4, 4, width, 30.0, env)
	assert.True(t, result)
}
