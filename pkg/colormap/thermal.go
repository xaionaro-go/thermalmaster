package colormap

import (
	"github.com/mazznoer/colorgrad"
)

// Ironbow returns the classic thermal camera colormap
// (black -> blue -> red -> orange -> yellow -> white).
func Ironbow() Colormap {
	grad, _ := colorgrad.NewGradient().
		HtmlColors("#00002a", "#09007a", "#4c00a8", "#8f0098", "#c4005a", "#e8351e", "#f97b1b", "#fcc31c", "#fcf075", "#ffffff").
		Build()
	return &gradientColormap{grad: grad, name: "ironbow"}
}

// Jet returns the rainbow/jet colormap (blue -> cyan -> green -> yellow -> red).
func Jet() Colormap {
	grad, _ := colorgrad.NewGradient().
		HtmlColors("#000080", "#0000ff", "#00ffff", "#00ff00", "#ffff00", "#ff0000", "#800000").
		Build()
	return &gradientColormap{grad: grad, name: "jet"}
}
