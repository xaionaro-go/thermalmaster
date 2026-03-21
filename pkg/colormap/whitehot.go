package colormap

import "image/color"

// WhiteHot returns a grayscale colormap where hot=white, cold=black.
func WhiteHot() Colormap { return &whiteHotColormap{} }

type whiteHotColormap struct{}

func (c *whiteHotColormap) At(t float64) color.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	v := uint8(t * 255)
	return color.RGBA{R: v, G: v, B: v, A: 255}
}

func (c *whiteHotColormap) String() string { return "whitehot" }
