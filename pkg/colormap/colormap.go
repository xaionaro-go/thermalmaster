package colormap

import "image/color"

// Colormap maps a normalized value [0.0, 1.0] to an RGBA color.
type Colormap interface {
	// At returns the color at normalized position t (0.0 = cold, 1.0 = hot).
	At(t float64) color.RGBA
}
