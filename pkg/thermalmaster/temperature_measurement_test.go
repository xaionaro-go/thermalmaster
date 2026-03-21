package thermalmaster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildThermalGrid creates a thermal data array where the raw value
// at pixel (x, y) equals baseRaw + RawThermalValue(x+y). This produces a known,
// spatially varying temperature pattern.
func buildThermalGrid(
	width, height int,
	baseRaw RawThermalValue,
) []RawThermalValue {
	data := make([]RawThermalValue, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			data[y*width+x] = baseRaw + RawThermalValue(x+y)
		}
	}
	return data
}

func TestPointTemp(t *testing.T) {
	width := 256
	height := 192
	baseRaw := RawThermalValue(19200)
	env := EnvParams{Emissivity: 1.0, ReflectedTemp: 25.0}
	thermal := buildThermalGrid(width, height, baseRaw)

	// At (0,0), raw = baseRaw = 19200 -> 300K -> 26.85C.
	temp := PointTemp(thermal, 0, 0, width, env)
	assert.InDelta(t, 26.85, temp, 0.02)

	// At (10,5), raw = 19200 + 15 = 19215 -> 19215/64 - 273.15.
	temp = PointTemp(thermal, 10, 5, width, env)
	expected := float64(19215)/TempScale - KelvinOffset
	assert.InDelta(t, expected, temp, 0.02)
}

func TestPointTempOutOfBounds(t *testing.T) {
	thermal := make([]RawThermalValue, 10)
	env := EnvParams{Emissivity: 1.0}
	assert.Equal(t, 0.0, PointTemp(thermal, 100, 100, 10, env))
}

func TestRectTemp(t *testing.T) {
	width := 256
	height := 192
	baseRaw := RawThermalValue(19200)
	env := EnvParams{Emissivity: 1.0, ReflectedTemp: 25.0}
	thermal := buildThermalGrid(width, height, baseRaw)

	// 4x4 rectangle at (0,0).
	info := RectTemp(thermal, 0, 0, 4, 4, width, env)

	// Min should be at (0,0) with raw=19200.
	minExpected := float64(baseRaw)/TempScale - KelvinOffset
	assert.InDelta(t, minExpected, info.Min, 0.02)
	assert.Equal(t, 0, info.MinX)
	assert.Equal(t, 0, info.MinY)

	// Max should be at (3,3) with raw=19200+6=19206.
	maxExpected := float64(baseRaw+6)/TempScale - KelvinOffset
	assert.InDelta(t, maxExpected, info.Max, 0.02)
	assert.Equal(t, 3, info.MaxX)
	assert.Equal(t, 3, info.MaxY)

	// Average should be between min and max.
	require.Greater(t, info.Avg, info.Min)
	require.Less(t, info.Avg, info.Max)
}

func TestLineTempHorizontal(t *testing.T) {
	width := 256
	height := 192
	baseRaw := RawThermalValue(19200)
	env := EnvParams{Emissivity: 1.0, ReflectedTemp: 25.0}
	thermal := buildThermalGrid(width, height, baseRaw)

	// Horizontal line at y=0 from x=0 to x=9.
	info := LineTemp(thermal, 0, 0, 9, 0, width, env)

	minExpected := float64(baseRaw)/TempScale - KelvinOffset
	assert.InDelta(t, minExpected, info.Min, 0.02)
	assert.Equal(t, 0, info.MinX)

	maxExpected := float64(baseRaw+9)/TempScale - KelvinOffset
	assert.InDelta(t, maxExpected, info.Max, 0.02)
	assert.Equal(t, 9, info.MaxX)
}

func TestLineTempVertical(t *testing.T) {
	width := 256
	height := 192
	baseRaw := RawThermalValue(19200)
	env := EnvParams{Emissivity: 1.0, ReflectedTemp: 25.0}
	thermal := buildThermalGrid(width, height, baseRaw)

	// Vertical line at x=0 from y=0 to y=9.
	info := LineTemp(thermal, 0, 0, 0, 9, width, env)

	minExpected := float64(baseRaw)/TempScale - KelvinOffset
	assert.InDelta(t, minExpected, info.Min, 0.02)
	assert.Equal(t, 0, info.MinY)

	maxExpected := float64(baseRaw+9)/TempScale - KelvinOffset
	assert.InDelta(t, maxExpected, info.Max, 0.02)
	assert.Equal(t, 9, info.MaxY)
}

func TestLineTempDiagonal(t *testing.T) {
	width := 256
	height := 192
	baseRaw := RawThermalValue(19200)
	env := EnvParams{Emissivity: 1.0, ReflectedTemp: 25.0}
	thermal := buildThermalGrid(width, height, baseRaw)

	// Diagonal line from (0,0) to (5,5).
	info := LineTemp(thermal, 0, 0, 5, 5, width, env)

	// Min at (0,0), raw=19200.
	minExpected := float64(baseRaw)/TempScale - KelvinOffset
	assert.InDelta(t, minExpected, info.Min, 0.02)
	assert.Equal(t, 0, info.MinX)
	assert.Equal(t, 0, info.MinY)

	// Max at (5,5), raw=19200+10=19210.
	maxExpected := float64(baseRaw+10)/TempScale - KelvinOffset
	assert.InDelta(t, maxExpected, info.Max, 0.02)
	assert.Equal(t, 5, info.MaxX)
	assert.Equal(t, 5, info.MaxY)
}
