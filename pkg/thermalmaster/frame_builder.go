package thermalmaster

import (
	"github.com/xaionaro-go/thermalmaster/pkg/colormap"
)

// FrameBuilderConfig holds parameters for building a frame from raw camera data.
type FrameBuilderConfig struct {
	Sensor   SensorSource
	Colormap colormap.Colormap  // nil = raw output (no colorization).
	Upscale  *UpscaleConfig     // nil = no upscaling (only used with SensorBlended).
}

// BuildPixels extracts sensor data from a raw camera frame, applies colorization
// and upscaling as configured, and returns the pixel data as bytes.
func BuildPixels(
	rawFrame []byte,
	modelCfg ModelConfig,
	cfg FrameBuilderConfig,
) (pixels []byte, format PixelFormat, width int, height int, thermal []RawThermalValue, ok bool) {
	width = modelCfg.SensorW
	height = modelCfg.SensorH

	switch cfg.Sensor {
	case SensorIR:
		ir := ExtractIRBrightness(rawFrame, modelCfg)
		if ir == nil {
			return nil, 0, 0, 0, nil, false
		}
		if cfg.Colormap != nil {
			pixels = ColorizeUint8(ir, cfg.Colormap)
			return pixels, PixelFormatRGB24, width, height, nil, true
		}
		return ir, PixelFormatGray8, width, height, nil, true

	case SensorBlended:
		ir, therm := ExtractBoth(rawFrame, modelCfg)
		if therm == nil {
			return nil, 0, 0, 0, nil, false
		}

		upCfg := DefaultUpscaleConfig()
		if cfg.Upscale != nil {
			upCfg = *cfg.Upscale
		}
		therm = JointBilateralUpsample(therm, ir, modelCfg.SensorW, modelCfg.SensorH, upCfg)
		width *= upCfg.Factor
		height *= upCfg.Factor

		if cfg.Colormap != nil {
			pixels, _, _ = ColorizeThermal(therm, cfg.Colormap)
			return pixels, PixelFormatRGB24, width, height, therm, true
		}
		return ThermalToBytes(therm), PixelFormatGray16LE, width, height, therm, true

	default: // SensorThermal
		therm := ExtractThermalData(rawFrame, modelCfg)
		if therm == nil {
			return nil, 0, 0, 0, nil, false
		}
		if cfg.Colormap != nil {
			pixels, _, _ = ColorizeThermal(therm, cfg.Colormap)
			return pixels, PixelFormatRGB24, width, height, therm, true
		}
		return ThermalToBytes(therm), PixelFormatGray16LE, width, height, therm, true
	}
}
