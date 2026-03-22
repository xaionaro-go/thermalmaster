package cliflags

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xaionaro-go/thermalmaster/pkg/colormap"
	"github.com/xaionaro-go/thermalmaster/pkg/thermalmaster"
)

// Config holds all CLI-configurable camera and rendering settings.
type Config struct {
	// Rendering.
	Sensor         string
	Colormap       string
	UpscaleFactor  int
	UpscaleWorkers int
	WindowRadius   int

	// Device selection.
	Serial     string
	USBBus     int
	USBAddr    int

	// Camera hardware.
	Gain       string
	HWPalette  string
	Brightness int
	Contrast   int
	MirrorFlip string

	// Startup.
	Shutter bool

	// Legend.
	Legend         bool
	LegendX        float64
	LegendY        float64
	LegendOrient   string
	LegendWidth    int
	LegendHeight   int
	LegendFontSz   float64
	LegendTempUnit string
}

// RegisterFlags registers all shared camera flags on a cobra command.
// Use -1 as sentinel for "not set" on integer hardware params.
func RegisterFlags(cmd *cobra.Command, cfg *Config) {
	// Rendering flags.
	cmd.Flags().StringVar(&cfg.Sensor, "sensor", "blended", "sensor source: thermal, ir, blended")
	cmd.Flags().StringVar(&cfg.Colormap, "colormap", "ironbow", "colormap name (none = raw)")
	cmd.Flags().IntVar(&cfg.UpscaleFactor, "upscale-factor", 2, "upscale factor for blended mode")
	cmd.Flags().IntVar(&cfg.UpscaleWorkers, "upscale-workers", 0, "parallel workers for upscaling (0 = single-threaded)")
	cmd.Flags().IntVar(&cfg.WindowRadius, "window-radius", 1, "bilateral filter half-window size (1 = 3x3, 2 = 5x5)")

	// Device selection flags.
	cmd.Flags().StringVar(&cfg.Serial, "serial", "", "select camera by serial number")
	cmd.Flags().IntVar(&cfg.USBBus, "usb-bus", -1, "select camera by USB bus number")
	cmd.Flags().IntVar(&cfg.USBAddr, "usb-addr", -1, "select camera by USB device address")

	// Camera hardware flags.
	cmd.Flags().StringVar(&cfg.Gain, "gain", "auto", "gain mode: auto, high, low")
	cmd.Flags().StringVar(&cfg.HWPalette, "hw-palette", "", "hardware palette: whitehot, blackhot, ironbow, rainbow, etc.")
	cmd.Flags().IntVar(&cfg.Brightness, "brightness", -1, "brightness level (0-100)")
	cmd.Flags().IntVar(&cfg.Contrast, "contrast", -1, "contrast level (0-100, uses global contrast)")
	cmd.Flags().StringVar(&cfg.MirrorFlip, "mirror-flip", "", "mirror/flip: none, mirror, flip, both")

	// Startup flags.
	cmd.Flags().BoolVar(&cfg.Shutter, "shutter", false, "trigger shutter calibration on startup")

	// Legend flags.
	cmd.Flags().BoolVar(&cfg.Legend, "legend", true, "enable legend overlay")
	cmd.Flags().Float64Var(&cfg.LegendX, "legend-x", 1.02, "legend X position as fraction of frame width")
	cmd.Flags().Float64Var(&cfg.LegendY, "legend-y", 0.05, "legend Y position as fraction of frame height")
	cmd.Flags().StringVar(&cfg.LegendOrient, "legend-orientation", "vertical", "legend orientation: vertical, horizontal")
	cmd.Flags().IntVar(&cfg.LegendWidth, "legend-width", 20, "legend bar width in pixels")
	cmd.Flags().IntVar(&cfg.LegendHeight, "legend-height", 0, "legend bar height in pixels (0 = 90%% of frame)")
	cmd.Flags().Float64Var(&cfg.LegendFontSz, "legend-font-size", 12, "legend font size in points")
	cmd.Flags().StringVar(&cfg.LegendTempUnit, "legend-temp-unit", "celsius", "temperature unit: celsius, fahrenheit, raw")
}

// ParseGain parses the gain flag. Returns (mode, isAuto, error).
func (c *Config) ParseGain() (thermalmaster.GainMode, bool, error) {
	return thermalmaster.ParseGainMode(c.Gain)
}

// ParseSensor parses the sensor flag.
func (c *Config) ParseSensor() (thermalmaster.SensorSource, error) {
	return thermalmaster.ParseSensorSource(c.Sensor)
}

// ParseColormap parses the colormap flag.
func (c *Config) ParseColormap() (colormap.Colormap, error) {
	return colormap.Parse(strings.ToLower(c.Colormap))
}

// BuildCameraConfig builds a CameraConfig from the parsed CLI flags.
// Only flags that were explicitly set (non-empty strings, non-sentinel ints)
// are included.
func (c *Config) BuildCameraConfig() (thermalmaster.CameraConfig, error) {
	var cc thermalmaster.CameraConfig

	if c.HWPalette != "" {
		v, err := thermalmaster.ParsePalette(c.HWPalette)
		if err != nil {
			return cc, err
		}
		cc.Palette = &v
	}

	if c.Brightness >= 0 {
		v := thermalmaster.BrightnessLevel(c.Brightness)
		cc.Brightness = &v
	}

	// The P3 APK's "contrast" SeekBar calls basicGlobalContrastLevelSet,
	// not basicImageContrastLevelSet. Map --contrast to GlobalContrast.
	if c.Contrast >= 0 {
		v := thermalmaster.ContrastLevel(c.Contrast)
		cc.GlobalContrast = &v
	}

	if c.MirrorFlip != "" {
		v, err := thermalmaster.ParseMirrorFlip(c.MirrorFlip)
		if err != nil {
			return cc, err
		}
		cc.MirrorFlip = &v
	}

	return cc, nil
}

// BuildUpscaleConfig builds an upscale config if the sensor is blended.
func (c *Config) BuildUpscaleConfig(
	sensor thermalmaster.SensorSource,
) *thermalmaster.UpscaleConfig {
	if sensor != thermalmaster.SensorBlended {
		return nil
	}

	uc := thermalmaster.DefaultUpscaleConfig()
	uc.Factor = c.UpscaleFactor
	uc.NumWorkers = c.UpscaleWorkers
	uc.WindowRadius = c.WindowRadius
	return &uc
}

// BuildLegendConfig builds a legend config from the CLI flags.
func (c *Config) BuildLegendConfig(
	cm colormap.Colormap,
) (thermalmaster.LegendConfig, error) {
	lcfg := thermalmaster.DefaultLegendConfig()
	lcfg.Enabled = c.Legend
	lcfg.X = c.LegendX
	lcfg.Y = c.LegendY
	lcfg.Width = c.LegendWidth
	lcfg.Height = c.LegendHeight
	lcfg.FontSize = c.LegendFontSz
	lcfg.Colormap = cm

	switch c.LegendOrient {
	case "horizontal":
		lcfg.Orientation = thermalmaster.LegendHorizontal
	default:
		lcfg.Orientation = thermalmaster.LegendVertical
	}

	switch c.LegendTempUnit {
	case "fahrenheit":
		lcfg.TempUnit = thermalmaster.TempFahrenheit
	case "raw":
		lcfg.TempUnit = thermalmaster.TempRaw
	default:
		lcfg.TempUnit = thermalmaster.TempCelsius
	}

	return lcfg, nil
}

// SetupCamera opens the camera, reads device info, and optionally triggers
// shutter calibration. Returns the device and device info (caller must Close
// the device). Hardware settings (gain, palette, etc.) should be applied AFTER
// streaming starts — some settings trigger ISP reconfiguration that disrupts
// the USB connection if applied before the streaming interface is claimed.
func (c *Config) SetupCamera() (_ *thermalmaster.Device, _ thermalmaster.DeviceInfo, _err error) {
	var openOpts []thermalmaster.OpenOption
	if c.Serial != "" {
		openOpts = append(openOpts, thermalmaster.WithSerial(c.Serial))
	}
	switch {
	case c.USBBus >= 0 && c.USBAddr >= 0:
		openOpts = append(openOpts, thermalmaster.WithUSBAddress(c.USBBus, c.USBAddr))
	case c.USBBus >= 0:
		openOpts = append(openOpts, thermalmaster.WithUSBBus(c.USBBus))
	}

	dev, err := thermalmaster.Open(openOpts...)
	if err != nil {
		return nil, thermalmaster.DeviceInfo{}, fmt.Errorf("opening P3: %w", err)
	}
	defer func() {
		if _err != nil {
			dev.Close()
		}
	}()

	info, err := dev.ReadDeviceInfo()
	if err != nil {
		return nil, thermalmaster.DeviceInfo{}, fmt.Errorf("reading device info: %w", err)
	}

	if c.Shutter {
		if err := dev.TriggerShutter(); err != nil {
			return nil, thermalmaster.DeviceInfo{}, fmt.Errorf("shutter trigger: %w", err)
		}
		time.Sleep(3 * time.Second)
	}

	return dev, info, nil
}

// ApplyHardwareSettings applies gain and other hardware settings to the device.
// Must be called AFTER StartStreaming — the camera ignores Set commands when
// the stream is not active. Each setting is verified via Get readback to
// confirm it took effect. Returns any accumulated warnings/errors.
func (c *Config) ApplyHardwareSettings(dev *thermalmaster.Device) error {
	var errs []error

	gain, _, err := c.ParseGain()
	if err != nil {
		errs = append(errs, err)
	} else {
		if err := dev.SetGain(gain); err != nil {
			errs = append(errs, fmt.Errorf("set gain: %w", err))
		}

		got, err := dev.GetGain()
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("verify gain: %w", err))
		case got != gain:
			errs = append(errs, fmt.Errorf("gain: set %d but read back %d", gain, got))
		}
	}

	camCfg, err := c.BuildCameraConfig()
	if err != nil {
		errs = append(errs, err)
		return errors.Join(errs...)
	}

	if err := camCfg.Apply(dev); err != nil {
		errs = append(errs, err)
	}

	// Verify each setting that was requested.
	if c.Brightness >= 0 {
		got, err := dev.GetBrightness()
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("verify brightness: %w", err))
		case got != thermalmaster.BrightnessLevel(c.Brightness):
			errs = append(errs, fmt.Errorf("brightness: set %d but read back %d", c.Brightness, got))
		}
	}

	if c.Contrast >= 0 {
		got, err := dev.GetGlobalContrast()
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("verify contrast: %w", err))
		case got != thermalmaster.ContrastLevel(c.Contrast):
			errs = append(errs, fmt.Errorf("contrast: set %d but read back %d", c.Contrast, got))
		}
	}

	if c.MirrorFlip != "" {
		want, _ := thermalmaster.ParseMirrorFlip(c.MirrorFlip)
		got, err := dev.GetMirrorFlip()
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("verify mirror-flip: %w", err))
		case got != want:
			errs = append(errs, fmt.Errorf("mirror-flip: set %d but read back %d", want, got))
		}
	}

	if c.HWPalette != "" {
		want, _ := thermalmaster.ParsePalette(c.HWPalette)
		got, err := dev.GetPalette()
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("verify palette: %w", err))
		case got != want:
			errs = append(errs, fmt.Errorf("palette: set %d but read back %d", want, got))
		}
	}

	return errors.Join(errs...)
}
