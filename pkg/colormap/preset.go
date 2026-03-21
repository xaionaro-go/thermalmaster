package colormap

import (
	"image/color"

	"github.com/mazznoer/colorgrad"
)

// gradientColormap wraps a colorgrad.Gradient as a Colormap.
type gradientColormap struct {
	grad colorgrad.Gradient
	name string
}

func (c *gradientColormap) At(t float64) color.RGBA {
	clr := c.grad.At(t)
	r, g, b, a := clr.RGBA255()
	return color.RGBA{R: r, G: g, B: b, A: a}
}

func (c *gradientColormap) String() string { return c.name }

func fromGrad(grad colorgrad.Gradient, name string) Colormap {
	return &gradientColormap{grad: grad, name: name}
}

// Perceptually uniform sequential colormaps.

func Inferno() Colormap  { return fromGrad(colorgrad.Inferno(), "inferno") }
func Viridis() Colormap  { return fromGrad(colorgrad.Viridis(), "viridis") }
func Magma() Colormap    { return fromGrad(colorgrad.Magma(), "magma") }
func Plasma() Colormap   { return fromGrad(colorgrad.Plasma(), "plasma") }
func Turbo() Colormap    { return fromGrad(colorgrad.Turbo(), "turbo") }
func Cividis() Colormap  { return fromGrad(colorgrad.Cividis(), "cividis") }
func Warm() Colormap     { return fromGrad(colorgrad.Warm(), "warm") }
func Cool() Colormap     { return fromGrad(colorgrad.Cool(), "cool") }

// Cubehelix colormaps.

func CubehelixDefault() Colormap { return fromGrad(colorgrad.CubehelixDefault(), "cubehelixDefault") }

// Sequential single-hue colormaps.

func Blues() Colormap   { return fromGrad(colorgrad.Blues(), "blues") }
func Greens() Colormap  { return fromGrad(colorgrad.Greens(), "greens") }
func Greys() Colormap   { return fromGrad(colorgrad.Greys(), "greys") }
func Oranges() Colormap { return fromGrad(colorgrad.Oranges(), "oranges") }
func Purples() Colormap { return fromGrad(colorgrad.Purples(), "purples") }
func Reds() Colormap    { return fromGrad(colorgrad.Reds(), "reds") }

// Sequential multi-hue colormaps.

func BuGn() Colormap  { return fromGrad(colorgrad.BuGn(), "buGn") }
func BuPu() Colormap  { return fromGrad(colorgrad.BuPu(), "buPu") }
func GnBu() Colormap  { return fromGrad(colorgrad.GnBu(), "gnBu") }
func OrRd() Colormap  { return fromGrad(colorgrad.OrRd(), "orRd") }
func PuBu() Colormap  { return fromGrad(colorgrad.PuBu(), "puBu") }
func PuBuGn() Colormap { return fromGrad(colorgrad.PuBuGn(), "puBuGn") }
func PuRd() Colormap  { return fromGrad(colorgrad.PuRd(), "puRd") }
func RdPu() Colormap  { return fromGrad(colorgrad.RdPu(), "rdPu") }
func YlGn() Colormap  { return fromGrad(colorgrad.YlGn(), "ylGn") }
func YlGnBu() Colormap { return fromGrad(colorgrad.YlGnBu(), "ylGnBu") }
func YlOrBr() Colormap { return fromGrad(colorgrad.YlOrBr(), "ylOrBr") }
func YlOrRd() Colormap { return fromGrad(colorgrad.YlOrRd(), "ylOrRd") }

// Diverging colormaps.

func BrBG() Colormap    { return fromGrad(colorgrad.BrBG(), "brBG") }
func PRGn() Colormap    { return fromGrad(colorgrad.PRGn(), "prGn") }
func PiYG() Colormap    { return fromGrad(colorgrad.PiYG(), "piYG") }
func PuOr() Colormap    { return fromGrad(colorgrad.PuOr(), "puOr") }
func RdBu() Colormap    { return fromGrad(colorgrad.RdBu(), "rdBu") }
func RdGy() Colormap    { return fromGrad(colorgrad.RdGy(), "rdGy") }
func RdYlBu() Colormap  { return fromGrad(colorgrad.RdYlBu(), "rdYlBu") }
func RdYlGn() Colormap  { return fromGrad(colorgrad.RdYlGn(), "rdYlGn") }
func Spectral() Colormap { return fromGrad(colorgrad.Spectral(), "spectral") }

// Cyclical colormaps.

func Rainbow() Colormap { return fromGrad(colorgrad.Rainbow(), "rainbow") }
func Sinebow() Colormap { return fromGrad(colorgrad.Sinebow(), "sinebow") }
