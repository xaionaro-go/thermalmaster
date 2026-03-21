package thermalmaster

// PixelFormat identifies the pixel encoding of a frame buffer.
type PixelFormat int

const (
	// PixelFormatGray8 is 8-bit grayscale (1 byte per pixel).
	PixelFormatGray8 PixelFormat = iota
	// PixelFormatGray16LE is 16-bit grayscale, little-endian (2 bytes per pixel).
	PixelFormatGray16LE
	// PixelFormatRGB24 is 24-bit RGB (3 bytes per pixel, no alpha).
	PixelFormatRGB24
	// PixelFormatRGBA32 is 32-bit RGBA (4 bytes per pixel).
	PixelFormatRGBA32
)

// BytesPerPixel returns the number of bytes per pixel for this format.
func (f PixelFormat) BytesPerPixel() int {
	switch f {
	case PixelFormatGray8:
		return 1
	case PixelFormatGray16LE:
		return 2
	case PixelFormatRGB24:
		return 3
	case PixelFormatRGBA32:
		return 4
	default:
		return 0
	}
}
