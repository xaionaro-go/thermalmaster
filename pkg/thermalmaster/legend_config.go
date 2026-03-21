package thermalmaster

import "github.com/xaionaro-go/thermalmaster/pkg/colormap"

// LegendConfig controls the legend overlay.
type LegendConfig struct {
	Enabled     bool
	X           float64            // position as fraction of frame width (>1.0 extends frame)
	Y           float64            // position as fraction of frame height (>1.0 extends frame)
	Orientation LegendOrientation
	Width       int                // bar width in pixels
	Height      int                // bar height in pixels (0 = 90% of frame height)
	FontSize    float64            // TrueType font size in points
	TempUnit    TempUnit
	Colormap    colormap.Colormap
}

// DefaultLegendConfig returns default legend parameters.
func DefaultLegendConfig() LegendConfig {
	return LegendConfig{
		X:           1.02,
		Y:           0.05,
		Orientation: LegendVertical,
		Width:       20,
		Height:      0,
		FontSize:    12,
		TempUnit:    TempCelsius,
	}
}
