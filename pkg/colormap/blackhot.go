package colormap

import "image/color"

// BlackHot returns a grayscale colormap where hot=black, cold=white.
func BlackHot() Colormap { return &blackHotColormap{} }

type blackHotColormap struct{}

func (c *blackHotColormap) At(t float64) color.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	v := uint8((1 - t) * 255)
	return color.RGBA{R: v, G: v, B: v, A: 255}
}

func (c *blackHotColormap) String() string { return "blackhot" }
