package thermalmaster

import (
	"fmt"
	"image"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const numLabels = 5

// LegendRenderer renders a color-mapped legend bar with temperature labels.
type LegendRenderer struct {
	cfg             LegendConfig
	fontFace        font.Face
	maxLabelWidth   int // precomputed worst-case label width for consistent output size
	gradientBar     *image.RGBA
	cachedBarHeight int
	cachedMin       RawThermalValue
	cachedMax       RawThermalValue
	cachedLabels    *image.RGBA
	outputBuf       *image.RGBA
}

// NewLegendRenderer creates a LegendRenderer from the given config.
func NewLegendRenderer(
	cfg LegendConfig,
) (*LegendRenderer, error) {
	f, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, fmt.Errorf("parsing Go Regular font: %w", err)
	}

	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    cfg.FontSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("creating font face: %w", err)
	}

	// Precompute worst-case label width so the legend always produces
	// consistent output dimensions regardless of actual temperature values.
	// Without this, the v4l2 format dimensions (computed from a dummy call)
	// can differ from real frames, causing every frame to be rejected.
	//
	// Measure the configured TempUnit at the extremes of RawThermalValue
	// rather than hardcoding strings — this stays correct for any unit
	// (current or future) and accounts for proportional font widths.
	maxLabelWidth := 0
	for _, raw := range []RawThermalValue{0, ^RawThermalValue(0)} {
		s := cfg.TempUnit.FormatValue(raw)
		w := font.MeasureString(face, s).Ceil()
		if w > maxLabelWidth {
			maxLabelWidth = w
		}
	}

	return &LegendRenderer{
		cfg:           cfg,
		fontFace:      face,
		maxLabelWidth: maxLabelWidth,
	}, nil
}

// renderGradientBar creates the gradient bar image.
//
// For vertical: Width = bar thickness, barHeight = bar length.
// Row 0 = hot (t=1.0), row H-1 = cold (t=0.0).
//
// For horizontal: Width = bar length, barHeight = bar thickness.
// Column 0 = cold (t=0.0), column W-1 = hot (t=1.0).
func (r *LegendRenderer) renderGradientBar(
	barHeight int,
) *image.RGBA {
	switch r.cfg.Orientation {
	case LegendHorizontal:
		barW := r.cfg.Width
		barH := barHeight

		bar := image.NewRGBA(image.Rect(0, 0, barW, barH))
		denomW := float64(barW - 1)
		if denomW == 0 {
			denomW = 1
		}
		for x := 0; x < barW; x++ {
			t := float64(x) / denomW
			c := r.cfg.Colormap.At(t)
			for y := 0; y < barH; y++ {
				bar.SetRGBA(x, y, c)
			}
		}
		return bar

	default: // LegendVertical
		w := r.cfg.Width
		h := barHeight

		bar := image.NewRGBA(image.Rect(0, 0, w, h))
		denomH := float64(h - 1)
		if denomH == 0 {
			denomH = 1
		}
		for y := 0; y < h; y++ {
			t := 1.0 - float64(y)/denomH
			c := r.cfg.Colormap.At(t)
			for x := 0; x < w; x++ {
				bar.SetRGBA(x, y, c)
			}
		}
		return bar
	}
}

// renderLabels creates an image with temperature text labels.
//
// For vertical: labels are arranged vertically beside the bar.
// Top = max (hot), bottom = min (cold).
//
// For horizontal: labels are arranged horizontally below the bar.
// Left = min (cold), right = max (hot).
func (r *LegendRenderer) renderLabels(
	barHeight int,
	tempMin RawThermalValue,
	tempMax RawThermalValue,
) *image.RGBA {
	metrics := r.fontFace.Metrics()
	ascent := metrics.Ascent.Ceil()
	lineHeight := (metrics.Ascent + metrics.Descent).Ceil()

	// Build label strings and find the widest one.
	labels := make([]string, numLabels)
	maxWidth := 0
	for i := 0; i < numLabels; i++ {
		t := float64(i) / float64(numLabels-1)
		raw := RawThermalValue(float64(tempMin) + t*float64(tempMax-tempMin))
		labels[i] = r.cfg.TempUnit.FormatValue(raw)

		w := font.MeasureString(r.fontFace, labels[i]).Ceil()
		if w > maxWidth {
			maxWidth = w
		}
	}

	switch r.cfg.Orientation {
	case LegendHorizontal:
		barW := r.cfg.Width
		img := image.NewRGBA(image.Rect(0, 0, barW, lineHeight))

		denomW := float64(barW - 1)
		if denomW == 0 {
			denomW = 1
		}
		for i := 0; i < numLabels; i++ {
			t := float64(i) / float64(numLabels-1)
			labelW := font.MeasureString(r.fontFace, labels[i]).Ceil()

			// i=0 is min (left), i=numLabels-1 is max (right).
			x := int(denomW * t)

			// Center the label horizontally around the target x position.
			x -= labelW / 2

			// Clamp so labels stay within the image bounds.
			if x < 0 {
				x = 0
			}
			if x+labelW > barW {
				x = barW - labelW
			}

			d := font.Drawer{
				Dst:  img,
				Src:  image.White,
				Face: r.fontFace,
				Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(ascent)},
			}
			d.DrawString(labels[i])
		}
		return img

	default: // LegendVertical
		// Pad the image vertically so text at the top and bottom edges
		// is not clipped. The pad accommodates the full line height
		// (ascent + descent) centered on each label position.
		pad := lineHeight/2 + 1
		imgH := barHeight + 2*pad
		// Use precomputed worst-case width so the output image size stays
		// constant across temperature ranges, avoiding v4l2 size mismatches.
		img := image.NewRGBA(image.Rect(0, 0, r.maxLabelWidth, imgH))

		denomH := float64(barHeight - 1)
		if denomH == 0 {
			denomH = 1
		}
		for i := 0; i < numLabels; i++ {
			t := float64(i) / float64(numLabels-1)
			// i=0 is min (bottom), i=numLabels-1 is max (top).
			// Invert so max is at y=0 (top of bar).
			y := int(denomH*(1.0-t)) + pad

			d := font.Drawer{
				Dst:  img,
				Src:  image.White,
				Face: r.fontFace,
				Dot:  fixed.Point26_6{X: 0, Y: fixed.I(y + ascent/2)},
			}
			d.DrawString(labels[i])
		}
		return img
	}
}

// blitPixels copies source pixel data into the destination RGBA image.
func blitPixels(
	dst *image.RGBA,
	src []byte,
	format PixelFormat,
	width int,
	height int,
) {
	bpp := format.BytesPerPixel()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			off := (y*width + x) * bpp
			dstOff := y*dst.Stride + x*4

			switch format {
			case PixelFormatRGBA32:
				dst.Pix[dstOff] = src[off]
				dst.Pix[dstOff+1] = src[off+1]
				dst.Pix[dstOff+2] = src[off+2]
				dst.Pix[dstOff+3] = src[off+3]
			case PixelFormatRGB24:
				dst.Pix[dstOff] = src[off]
				dst.Pix[dstOff+1] = src[off+1]
				dst.Pix[dstOff+2] = src[off+2]
				dst.Pix[dstOff+3] = 255
			default:
				// Gray formats: replicate the first byte across RGB.
				dst.Pix[dstOff] = src[off]
				dst.Pix[dstOff+1] = src[off]
				dst.Pix[dstOff+2] = src[off]
				dst.Pix[dstOff+3] = 255
			}
		}
	}
}

// Apply composites the legend overlay onto the given pixel buffer and returns
// the resulting RGBA image. If the legend is disabled, it returns nil.
// The legend position may extend the output image beyond the input dimensions.
//
// The returned image is reused across calls — callers must not retain the
// pointer beyond the next Apply call. Copy the pixel data if needed.
func (r *LegendRenderer) Apply(
	pixels []byte,
	format PixelFormat,
	width int,
	height int,
	tempMin RawThermalValue,
	tempMax RawThermalValue,
) *image.RGBA {
	if !r.cfg.Enabled {
		return nil
	}

	// Compute effective bar height.
	barHeight := r.cfg.Height
	if barHeight <= 0 {
		barHeight = int(float64(height) * 0.9)
	}

	// Compute legend position.
	legendX := int(r.cfg.X * float64(width))
	legendY := int(r.cfg.Y * float64(height))

	// Render or reuse cached gradient bar.
	if r.gradientBar == nil || r.cachedBarHeight != barHeight {
		r.gradientBar = r.renderGradientBar(barHeight)
		r.cachedBarHeight = barHeight
		// Bar height changed, so labels need re-rendering too.
		r.cachedLabels = nil
	}
	bar := r.gradientBar

	// Render or reuse cached labels.
	if tempMin != r.cachedMin || tempMax != r.cachedMax || r.cachedLabels == nil {
		r.cachedLabels = r.renderLabels(barHeight, tempMin, tempMax)
		r.cachedMin = tempMin
		r.cachedMax = tempMax
	}
	labels := r.cachedLabels

	// Compute legend total extent based on orientation.
	var legendTotalW, legendTotalH int
	switch r.cfg.Orientation {
	case LegendHorizontal:
		// Labels are below the bar.
		legendTotalW = max(bar.Bounds().Dx(), labels.Bounds().Dx())
		legendTotalH = bar.Bounds().Dy() + labels.Bounds().Dy()
	default: // LegendVertical
		// Labels are to the right of the bar. Labels image includes padding
		// that extends above and below the bar.
		legendTotalW = bar.Bounds().Dx() + labels.Bounds().Dx()
		legendTotalH = labels.Bounds().Dy()
	}

	// Compute output dimensions (may extend beyond input frame).
	outW := max(width, legendX+legendTotalW)
	outH := max(height, legendY+legendTotalH)

	// Allocate or reuse output buffer.
	if r.outputBuf == nil ||
		r.outputBuf.Bounds().Dx() != outW ||
		r.outputBuf.Bounds().Dy() != outH {
		r.outputBuf = image.NewRGBA(image.Rect(0, 0, outW, outH))
	} else {
		// Clear the buffer.
		clear(r.outputBuf.Pix)
	}

	// Blit input pixels into output buffer.
	blitPixels(r.outputBuf, pixels, format, width, height)

	// Draw gradient bar.
	draw.Draw(
		r.outputBuf,
		image.Rect(legendX, legendY, legendX+bar.Bounds().Dx(), legendY+bar.Bounds().Dy()),
		bar,
		image.Point{},
		draw.Over,
	)

	// Draw labels.
	switch r.cfg.Orientation {
	case LegendHorizontal:
		// Labels below the bar.
		draw.Draw(
			r.outputBuf,
			image.Rect(
				legendX,
				legendY+bar.Bounds().Dy(),
				legendX+labels.Bounds().Dx(),
				legendY+bar.Bounds().Dy()+labels.Bounds().Dy(),
			),
			labels,
			image.Point{},
			draw.Over,
		)
	default: // LegendVertical
		// Labels to the right of the bar. The labels image has vertical
		// padding so text at top/bottom edges is not clipped. Offset the
		// labels upward by the padding amount to align with the bar.
		metrics := r.fontFace.Metrics()
		labelLineH := (metrics.Ascent + metrics.Descent).Ceil()
		labelPad := labelLineH/2 + 1
		labelsY := legendY - labelPad

		draw.Draw(
			r.outputBuf,
			image.Rect(
				legendX+bar.Bounds().Dx(),
				labelsY,
				legendX+bar.Bounds().Dx()+labels.Bounds().Dx(),
				labelsY+labels.Bounds().Dy(),
			),
			labels,
			image.Point{},
			draw.Over,
		)
	}

	return r.outputBuf
}
