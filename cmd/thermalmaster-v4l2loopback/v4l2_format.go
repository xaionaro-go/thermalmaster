package main

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/xaionaro-go/thermalmaster/pkg/colormap"
	"github.com/xaionaro-go/thermalmaster/pkg/thermalmaster"
	"golang.org/x/sys/unix"
)

// v4l2PixFmtFor returns the v4l2 pixel format fourcc for the given sensor and colormap.
func v4l2PixFmtFor(sensor thermalmaster.SensorSource, cm colormap.Colormap) uint32 {
	if cm != nil {
		return pixFmtRGB24
	}
	switch sensor {
	case thermalmaster.SensorIR:
		return pixFmtGrey
	default:
		return pixFmtY16
	}
}

// v4l2 pixel format fourcc codes.
const (
	pixFmtGrey  = uint32('G') | uint32('R')<<8 | uint32('E')<<16 | uint32('Y')<<24
	pixFmtY16   = uint32('Y') | uint32('1')<<8 | uint32('6')<<16 | uint32(' ')<<24
	pixFmtRGB24 = uint32('R') | uint32('G')<<8 | uint32('B')<<16 | uint32('3')<<24
)

const (
	v4l2BufTypeVideoOutput = 2
	v4l2FieldNone          = 1
	v4l2ColorspaceSRGB     = 8

	// VIDIOC_S_FMT = _IOWR('V', 5, struct v4l2_format) = 0xc0d05605
	ioctlSetFmt = 0xc0d05605
)

// v4l2PixFormat mirrors the kernel's struct v4l2_pix_format (48 bytes).
type v4l2PixFormat struct {
	Width        uint32
	Height       uint32
	PixelFormat  uint32
	Field        uint32
	BytesPerLine uint32
	SizeImage    uint32
	Colorspace   uint32
	Priv         uint32
	Flags        uint32
	YCbCrEnc     uint32
	Quantization uint32
	XferFunc     uint32
}

// v4l2Format mirrors the kernel's struct v4l2_format (208 bytes).
type v4l2Format struct {
	Type uint32
	_    [4]byte // padding between type and the fmt union
	Pix  v4l2PixFormat
	_    [200 - 48]byte // pad union to 200 bytes (raw_data size)
}

// setV4L2Format configures the pixel format on a v4l2loopback output device.
func setV4L2Format(
	f *os.File,
	width uint32,
	height uint32,
	pixelFormat uint32,
	bytesPerPixel uint32,
) error {
	v4l2Fmt := v4l2Format{
		Type: v4l2BufTypeVideoOutput,
		Pix: v4l2PixFormat{
			Width:        width,
			Height:       height,
			PixelFormat:  pixelFormat,
			Field:        v4l2FieldNone,
			BytesPerLine: width * bytesPerPixel,
			SizeImage:    width * height * bytesPerPixel,
			Colorspace:   v4l2ColorspaceSRGB,
		},
	}

	if _, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		f.Fd(),
		ioctlSetFmt,
		uintptr(unsafe.Pointer(&v4l2Fmt)),
	); errno != 0 {
		return fmt.Errorf("VIDIOC_S_FMT: %w", errno)
	}
	return nil
}
