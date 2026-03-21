package thermalmaster

import (
	"encoding/binary"
	"image"
)

// ThermalToBytes serializes thermal values to GRAY16LE bytes (2 bytes per pixel,
// little-endian).
func ThermalToBytes(thermal []RawThermalValue) []byte {
	out := make([]byte, len(thermal)*2)
	for i, v := range thermal {
		binary.LittleEndian.PutUint16(out[i*2:], uint16(v))
	}
	return out
}

// RGBAToRGB24 converts an RGBA image to a packed RGB24 byte stream
// (3 bytes per pixel), discarding the alpha channel.
func RGBAToRGB24(img *image.RGBA) []byte {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	out := make([]byte, w*h*3)
	for y := range h {
		for x := range w {
			srcOff := y*img.Stride + x*4
			dstOff := (y*w + x) * 3
			out[dstOff] = img.Pix[srcOff]
			out[dstOff+1] = img.Pix[srcOff+1]
			out[dstOff+2] = img.Pix[srcOff+2]
		}
	}
	return out
}

// ThermalMinMax returns the minimum and maximum values in a thermal slice.
func ThermalMinMax(
	thermal []RawThermalValue,
) (RawThermalValue, RawThermalValue) {
	if len(thermal) == 0 {
		return 0, 0
	}

	min, max := thermal[0], thermal[0]
	for _, v := range thermal[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}
