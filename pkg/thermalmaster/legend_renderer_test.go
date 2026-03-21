package thermalmaster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xaionaro-go/thermalmaster/pkg/colormap"
)

func TestLegendRenderer_GradientBar(t *testing.T) {
	cm, err := colormap.Parse("ironbow")
	require.NoError(t, err)

	cfg := DefaultLegendConfig()
	cfg.Enabled = true
	cfg.Colormap = cm
	cfg.Width = 20
	cfg.Height = 100

	r, err := NewLegendRenderer(cfg)
	require.NoError(t, err)

	bar := r.renderGradientBar(cfg.Height)
	assert.Equal(t, 20, bar.Bounds().Dx())
	assert.Equal(t, 100, bar.Bounds().Dy())

	// Top of vertical bar = hot (t=1.0), bottom = cold (t=0.0)
	topColor := bar.RGBAAt(10, 0)
	botColor := bar.RGBAAt(10, 99)
	expectedTop := cm.At(1.0)
	expectedBot := cm.At(0.0)
	assert.Equal(t, expectedTop.R, topColor.R)
	assert.Equal(t, expectedBot.R, botColor.R)
}

func TestLegendRenderer_GradientBar_Horizontal(t *testing.T) {
	cm, err := colormap.Parse("ironbow")
	require.NoError(t, err)

	cfg := DefaultLegendConfig()
	cfg.Enabled = true
	cfg.Colormap = cm
	cfg.Orientation = LegendHorizontal
	cfg.Width = 100
	cfg.Height = 20

	r, err := NewLegendRenderer(cfg)
	require.NoError(t, err)

	bar := r.renderGradientBar(20)
	assert.Equal(t, 100, bar.Bounds().Dx())
	assert.Equal(t, 20, bar.Bounds().Dy())

	leftColor := bar.RGBAAt(0, 10)
	rightColor := bar.RGBAAt(99, 10)
	assert.Equal(t, cm.At(0.0).R, leftColor.R)
	assert.Equal(t, cm.At(1.0).R, rightColor.R)
}

func TestLegendRenderer_Labels(t *testing.T) {
	cm, err := colormap.Parse("ironbow")
	require.NoError(t, err)

	cfg := DefaultLegendConfig()
	cfg.Enabled = true
	cfg.Colormap = cm
	cfg.Width = 20
	cfg.Height = 100
	cfg.TempUnit = TempCelsius

	r, err := NewLegendRenderer(cfg)
	require.NoError(t, err)

	// 19200 raw ≈ 26.85°C (300K), 25600 raw ≈ 126.85°C (400K)
	labels := r.renderLabels(cfg.Height, RawThermalValue(19200), RawThermalValue(25600))
	require.NotNil(t, labels)
	assert.Greater(t, labels.Bounds().Dx(), 0)
	assert.Greater(t, labels.Bounds().Dy(), 0)

	// The image height includes padding for text at top/bottom edges.
	assert.GreaterOrEqual(t, labels.Bounds().Dy(), cfg.Height, "label image must be at least bar height")

	// Verify labels contain non-transparent pixels (text was actually drawn).
	hasNonTransparent := false
	bounds := labels.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y && !hasNonTransparent; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := labels.At(x, y).RGBA()
			if a > 0 {
				hasNonTransparent = true
				break
			}
		}
	}
	assert.True(t, hasNonTransparent, "label image must contain non-transparent pixels (drawn text)")
}

func TestLegendRenderer_Labels_DifferentUnits(t *testing.T) {
	cm, err := colormap.Parse("ironbow")
	require.NoError(t, err)

	for _, unit := range []TempUnit{TempCelsius, TempFahrenheit, TempRaw} {
		t.Run(unit.FormatValue(19200), func(t *testing.T) {
			cfg := DefaultLegendConfig()
			cfg.Enabled = true
			cfg.Colormap = cm
			cfg.Width = 20
			cfg.Height = 100
			cfg.TempUnit = unit

			r, err := NewLegendRenderer(cfg)
			require.NoError(t, err)

			labels := r.renderLabels(cfg.Height, RawThermalValue(19200), RawThermalValue(25600))
			require.NotNil(t, labels)
			assert.Greater(t, labels.Bounds().Dx(), 0)
			assert.Greater(t, labels.Bounds().Dy(), 0)
		})
	}
}

func TestLegendRenderer_Apply_NoExtension(t *testing.T) {
	cm, err := colormap.Parse("ironbow")
	require.NoError(t, err)

	cfg := DefaultLegendConfig()
	cfg.Enabled = true
	cfg.Colormap = cm
	cfg.X = 0.8
	cfg.Y = 0.1
	cfg.Width = 10
	cfg.Height = 50

	r, err := NewLegendRenderer(cfg)
	require.NoError(t, err)

	// 100x100 RGB24 frame
	pixels := make([]byte, 100*100*3)
	// Fill with red
	for i := 0; i < len(pixels); i += 3 {
		pixels[i] = 255
	}
	result := r.Apply(pixels, PixelFormatRGB24, 100, 100, 19200, 25600)
	require.NotNil(t, result)
	// Check original pixels are preserved
	assert.Equal(t, uint8(255), result.RGBAAt(0, 0).R)
}

func TestLegendRenderer_Apply_WithExtension(t *testing.T) {
	cm, err := colormap.Parse("ironbow")
	require.NoError(t, err)

	cfg := DefaultLegendConfig()
	cfg.Enabled = true
	cfg.Colormap = cm
	cfg.X = 1.05
	cfg.Y = 0.1
	cfg.Width = 20
	cfg.Height = 80

	r, err := NewLegendRenderer(cfg)
	require.NoError(t, err)

	pixels := make([]byte, 100*100*3)
	result := r.Apply(pixels, PixelFormatRGB24, 100, 100, 19200, 25600)
	require.NotNil(t, result)
	assert.Greater(t, result.Bounds().Dx(), 100)
}

func TestLegendRenderer_Apply_Disabled(t *testing.T) {
	cfg := DefaultLegendConfig()
	cfg.Enabled = false

	r, err := NewLegendRenderer(cfg)
	require.NoError(t, err)

	pixels := make([]byte, 100*100*3)
	result := r.Apply(pixels, PixelFormatRGB24, 100, 100, 19200, 25600)
	assert.Nil(t, result)
}

func TestLegendRenderer_Apply_CachesLabels(t *testing.T) {
	cm, err := colormap.Parse("ironbow")
	require.NoError(t, err)

	cfg := DefaultLegendConfig()
	cfg.Enabled = true
	cfg.Colormap = cm
	cfg.Width = 10
	cfg.Height = 50

	r, err := NewLegendRenderer(cfg)
	require.NoError(t, err)

	pixels := make([]byte, 100*100*3)
	// First call
	r.Apply(pixels, PixelFormatRGB24, 100, 100, 19200, 25600)
	// Second call with same min/max — should reuse cached labels
	r.Apply(pixels, PixelFormatRGB24, 100, 100, 19200, 25600)
	// Third call with different min/max — should re-render labels
	r.Apply(pixels, PixelFormatRGB24, 100, 100, 19200, 26000)
	// All should succeed without panic
}
