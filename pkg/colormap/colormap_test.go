package colormap

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPresets(t *testing.T) {
	presets := []struct {
		name string
		cm   Colormap
	}{
		{"Inferno", Inferno()},
		{"Viridis", Viridis()},
		{"Magma", Magma()},
		{"Plasma", Plasma()},
		{"Turbo", Turbo()},
		{"Cividis", Cividis()},
		{"Ironbow", Ironbow()},
		{"Jet", Jet()},
		{"WhiteHot", WhiteHot()},
		{"BlackHot", BlackHot()},
	}

	for _, p := range presets {
		t.Run(p.name, func(t *testing.T) {
			c0 := p.cm.At(0.0)
			assert.Equal(t, uint8(255), c0.A, "At(0.0) should have A=255")

			c05 := p.cm.At(0.5)
			assert.Equal(t, uint8(255), c05.A, "At(0.5) should have A=255")

			c1 := p.cm.At(1.0)
			assert.Equal(t, uint8(255), c1.A, "At(1.0) should have A=255")

			// At(0.0) and At(0.5) should generally differ.
			assert.True(t, c0 != c05,
				"At(0.0) and At(0.5) should produce different colors for %s", p.name)
		})
	}
}

func TestWhiteHotEndpoints(t *testing.T) {
	cm := WhiteHot()
	c0 := cm.At(0.0)
	assert.Equal(t, color.RGBA{R: 0, G: 0, B: 0, A: 255}, c0)

	c1 := cm.At(1.0)
	assert.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, c1)
}

func TestBlackHotEndpoints(t *testing.T) {
	cm := BlackHot()
	c0 := cm.At(0.0)
	assert.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, c0)

	c1 := cm.At(1.0)
	assert.Equal(t, color.RGBA{R: 0, G: 0, B: 0, A: 255}, c1)
}

func TestClampingBelowZero(t *testing.T) {
	cm := WhiteHot()
	cNeg := cm.At(-0.5)
	c0 := cm.At(0.0)
	assert.Equal(t, c0, cNeg, "At(-0.5) should clamp to At(0.0)")
}

func TestClampingAboveOne(t *testing.T) {
	cm := WhiteHot()
	cOver := cm.At(1.5)
	c1 := cm.At(1.0)
	assert.Equal(t, c1, cOver, "At(1.5) should clamp to At(1.0)")
}

func TestFromHTMLColors_Valid(t *testing.T) {
	cm, err := FromHTMLColors("#000000", "#ff0000", "#ffffff")
	require.NoError(t, err)

	c0 := cm.At(0.0)
	assert.Equal(t, uint8(255), c0.A)

	c1 := cm.At(1.0)
	assert.Equal(t, uint8(255), c1.A)

	// Midpoint should be reddish.
	c05 := cm.At(0.5)
	assert.True(t, c05.R > c05.G, "midpoint should be reddish: R=%d, G=%d", c05.R, c05.G)
}

func TestFromHTMLColors_Invalid(t *testing.T) {
	_, err := FromHTMLColors("#000000", "not-a-color", "#ffffff")
	assert.Error(t, err)
}

func TestFromColors(t *testing.T) {
	cm := FromColors(
		color.RGBA{R: 0, G: 0, B: 0, A: 255},
		color.RGBA{R: 255, G: 255, B: 255, A: 255},
	)

	c0 := cm.At(0.0)
	assert.Equal(t, color.RGBA{R: 0, G: 0, B: 0, A: 255}, c0)

	c1 := cm.At(1.0)
	assert.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, c1)

	// Midpoint should be ~127-128.
	c05 := cm.At(0.5)
	assert.InDelta(t, 127.5, float64(c05.R), 1.0, "midpoint R should be ~127-128")
	assert.InDelta(t, 127.5, float64(c05.G), 1.0, "midpoint G should be ~127-128")
	assert.InDelta(t, 127.5, float64(c05.B), 1.0, "midpoint B should be ~127-128")
	assert.Equal(t, uint8(255), c05.A)
}

func TestFromColors_Empty(t *testing.T) {
	cm := FromColors()
	c := cm.At(0.5)
	assert.Equal(t, color.RGBA{}, c)
}

func TestFromColors_SingleColor(t *testing.T) {
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	cm := FromColors(red)

	// With a single color, every position should return that color.
	c0 := cm.At(0.0)
	assert.Equal(t, red, c0)

	c1 := cm.At(1.0)
	assert.Equal(t, red, c1)
}
