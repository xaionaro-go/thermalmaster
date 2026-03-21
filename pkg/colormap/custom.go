package colormap

import (
	"image/color"

	"github.com/mazznoer/colorgrad"
)

// FromHTMLColors creates a custom colormap from a list of HTML color strings.
// Colors are evenly distributed from 0.0 to 1.0.
func FromHTMLColors(colors ...string) (Colormap, error) {
	grad, err := colorgrad.NewGradient().HtmlColors(colors...).Build()
	if err != nil {
		return nil, err
	}
	return &gradientColormap{grad: grad, name: "custom"}, nil
}

// FromColors creates a custom colormap from Go color.Color values.
// Colors are linearly interpolated across the [0, 1] range.
func FromColors(colors ...color.Color) Colormap {
	return &interpolatedColormap{colors: colors}
}

type interpolatedColormap struct {
	colors []color.Color
}

func (c *interpolatedColormap) At(t float64) color.RGBA {
	if len(c.colors) == 0 {
		return color.RGBA{}
	}

	if t <= 0 {
		r, g, b, a := c.colors[0].RGBA()
		return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	}
	if t >= 1 {
		last := c.colors[len(c.colors)-1]
		r, g, b, a := last.RGBA()
		return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	}

	pos := t * float64(len(c.colors)-1)
	idx := int(pos)
	frac := pos - float64(idx)

	if idx >= len(c.colors)-1 {
		r, g, b, a := c.colors[len(c.colors)-1].RGBA()
		return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	}

	r1, g1, b1, _ := c.colors[idx].RGBA()
	r2, g2, b2, _ := c.colors[idx+1].RGBA()

	return color.RGBA{
		R: uint8(float64(r1>>8)*(1-frac) + float64(r2>>8)*frac),
		G: uint8(float64(g1>>8)*(1-frac) + float64(g2>>8)*frac),
		B: uint8(float64(b1>>8)*(1-frac) + float64(b2>>8)*frac),
		A: 255,
	}
}
