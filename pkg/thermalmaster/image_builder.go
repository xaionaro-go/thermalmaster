package thermalmaster

import (
	"encoding/binary"
	"image"
)

// BuildImage is like BuildPixels but wraps the result in an image.Image.
func BuildImage(
	rawFrame []byte,
	modelCfg ModelConfig,
	cfg FrameBuilderConfig,
) (img image.Image, thermal []RawThermalValue, ok bool) {
	pixels, format, width, height, thermal, ok := BuildPixels(rawFrame, modelCfg, cfg)
	if !ok {
		return nil, nil, false
	}

	switch format {
	case PixelFormatRGB24:
		rgba := image.NewRGBA(image.Rect(0, 0, width, height))
		for i := range width * height {
			rgba.Pix[i*4] = pixels[i*3]
			rgba.Pix[i*4+1] = pixels[i*3+1]
			rgba.Pix[i*4+2] = pixels[i*3+2]
			rgba.Pix[i*4+3] = 255
		}
		return rgba, thermal, true

	case PixelFormatGray8:
		gray := image.NewGray(image.Rect(0, 0, width, height))
		gray.Pix = pixels
		return gray, thermal, true

	case PixelFormatGray16LE:
		gray16 := image.NewGray16(image.Rect(0, 0, width, height))
		for i := range width * height {
			// image.Gray16 stores big-endian.
			le := binary.LittleEndian.Uint16(pixels[i*2:])
			binary.BigEndian.PutUint16(gray16.Pix[i*2:], le)
		}
		return gray16, thermal, true

	default:
		return nil, nil, false
	}
}
