package thermalmaster

import (
	"errors"
	"fmt"
	"strings"
)

// CameraConfig holds user-configurable hardware settings for the camera.
// All pointer fields are optional — nil means "don't change from current".
type CameraConfig struct {
	Gain           *GainMode
	SceneMode      *SceneMode
	ProfessionMode *ProfessionMode
	EdgeEnhance    *EdgeEnhanceLevel
	Palette        *PaletteIndex
	ROILevel       *ROILevel
	NoiseReduction *NoiseReductionLevel
	DetailEnhance  *DetailEnhanceLevel
	Brightness     *BrightnessLevel
	Contrast       *ContrastLevel
	GlobalContrast *ContrastLevel
	MirrorFlip     *MirrorFlipMode
}

// Apply sends all non-nil settings to the device. Each Set method
// internally verifies the value by reading it back.
func (cc CameraConfig) Apply(dev *Device) error {
	var errs []error

	if cc.Gain != nil {
		if err := dev.SetGain(*cc.Gain); err != nil {
			errs = append(errs, err)
		}
	}
	if cc.SceneMode != nil {
		if err := dev.SetSceneMode(*cc.SceneMode); err != nil {
			errs = append(errs, err)
		}
	}
	if cc.ProfessionMode != nil {
		if err := dev.SetProfessionMode(*cc.ProfessionMode); err != nil {
			errs = append(errs, err)
		}
	}
	if cc.EdgeEnhance != nil {
		if err := dev.SetEdgeEnhance(*cc.EdgeEnhance); err != nil {
			errs = append(errs, err)
		}
	}
	if cc.Palette != nil {
		if err := dev.SetPalette(*cc.Palette); err != nil {
			errs = append(errs, err)
		}
	}
	if cc.ROILevel != nil {
		if err := dev.SetROILevel(*cc.ROILevel); err != nil {
			errs = append(errs, err)
		}
	}
	if cc.NoiseReduction != nil {
		if err := dev.SetNoiseReduction(*cc.NoiseReduction); err != nil {
			errs = append(errs, err)
		}
	}
	if cc.DetailEnhance != nil {
		if err := dev.SetDetailEnhance(*cc.DetailEnhance); err != nil {
			errs = append(errs, err)
		}
	}
	if cc.Brightness != nil {
		if err := dev.SetBrightness(*cc.Brightness); err != nil {
			errs = append(errs, err)
		}
	}
	if cc.Contrast != nil {
		if err := dev.SetContrast(*cc.Contrast); err != nil {
			errs = append(errs, err)
		}
	}
	if cc.GlobalContrast != nil {
		if err := dev.SetGlobalContrast(*cc.GlobalContrast); err != nil {
			errs = append(errs, err)
		}
	}
	if cc.MirrorFlip != nil {
		if err := dev.SetMirrorFlip(*cc.MirrorFlip); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// ParseGainMode parses a gain mode string.
// Valid values: "auto", "high", "low".
func ParseGainMode(s string) (GainMode, bool, error) {
	switch strings.ToLower(s) {
	case "auto":
		return GainHigh, true, nil
	case "high":
		return GainHigh, false, nil
	case "low":
		return GainLow, false, nil
	default:
		return 0, false, fmt.Errorf("unknown gain: %q (use: auto, high, low)", s)
	}
}

// ParseSceneMode parses a scene mode string.
func ParseSceneMode(s string) (SceneMode, error) {
	switch strings.ToLower(s) {
	case "normal":
		return SceneNormal, nil
	case "city":
		return SceneCity, nil
	case "jungle":
		return SceneJungle, nil
	case "bird":
		return SceneBird, nil
	case "normal50":
		return SceneNormal50, nil
	case "city50":
		return SceneCity50, nil
	case "jungle50":
		return SceneJungle50, nil
	case "bird50":
		return SceneBird50, nil
	case "rainfog":
		return SceneRainFog, nil
	default:
		return 0, fmt.Errorf("unknown scene: %q (use: normal, city, jungle, bird, rainfog, normal50, city50, jungle50, bird50)", s)
	}
}

// ParseProfessionMode parses a profession mode string.
func ParseProfessionMode(s string) (ProfessionMode, error) {
	switch strings.ToLower(s) {
	case "normal":
		return ProfessionNormal, nil
	case "professional":
		return ProfessionProfessional, nil
	default:
		return 0, fmt.Errorf("unknown profession mode: %q (use: normal, professional)", s)
	}
}

// ParseEdgeEnhance parses an edge enhance level string.
func ParseEdgeEnhance(s string) (EdgeEnhanceLevel, error) {
	switch strings.ToLower(s) {
	case "off", "0":
		return EdgeEnhanceOff, nil
	case "1":
		return EdgeEnhanceLevel1, nil
	case "2":
		return EdgeEnhanceLevel2, nil
	default:
		return 0, fmt.Errorf("unknown edge enhance: %q (use: off, 1, 2)", s)
	}
}

// ParsePalette parses a hardware palette name string.
func ParsePalette(s string) (PaletteIndex, error) {
	switch strings.ToLower(s) {
	case "whitehot":
		return PaletteWhiteHot, nil
	case "blackhot":
		return PaletteBlackHot, nil
	case "rainbow":
		return PaletteRainbow, nil
	case "ironbow":
		return PaletteIronbow, nil
	case "aurora":
		return PaletteAurora, nil
	case "jungle":
		return PaletteJungle, nil
	case "gloryhot":
		return PaletteGloryHot, nil
	case "medical":
		return PaletteMedical, nil
	case "night":
		return PaletteNight, nil
	case "sepia":
		return PaletteSepia, nil
	case "redhot":
		return PaletteRedHot, nil
	default:
		return 0, fmt.Errorf("unknown palette: %q (use: whitehot, blackhot, rainbow, ironbow, aurora, jungle, gloryhot, medical, night, sepia, redhot)", s)
	}
}

// ParseROILevel parses an ROI level string.
func ParseROILevel(s string) (ROILevel, error) {
	switch strings.ToLower(s) {
	case "off", "disable":
		return ROIDisable, nil
	case "third":
		return ROIThird, nil
	case "half":
		return ROIHalf, nil
	default:
		return 0, fmt.Errorf("unknown ROI level: %q (use: off, third, half)", s)
	}
}

// ParseMirrorFlip parses a mirror/flip mode string.
func ParseMirrorFlip(s string) (MirrorFlipMode, error) {
	switch strings.ToLower(s) {
	case "none":
		return MirrorFlipNone, nil
	case "mirror":
		return MirrorOnly, nil
	case "flip":
		return FlipOnly, nil
	case "both":
		return MirrorAndFlip, nil
	default:
		return 0, fmt.Errorf("unknown mirror/flip: %q (use: none, mirror, flip, both)", s)
	}
}
