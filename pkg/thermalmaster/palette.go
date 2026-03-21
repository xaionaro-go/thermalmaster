package thermalmaster

import "fmt"

// PaletteIndex represents a color palette index.
//
// The P3 uses the "USB Dual" palette numbering (values 16-26).
// Other device families may use different numbering schemes.
type PaletteIndex uint8

// Hardware palettes for P3 (USB Dual numbering).
const (
	PaletteWhiteHot PaletteIndex = 16
	PaletteBlackHot PaletteIndex = 17
	PaletteRainbow  PaletteIndex = 18
	PaletteIronbow  PaletteIndex = 19
	PaletteAurora   PaletteIndex = 20
	PaletteJungle   PaletteIndex = 21
	PaletteGloryHot PaletteIndex = 22
	PaletteMedical  PaletteIndex = 23
	PaletteNight    PaletteIndex = 24
	PaletteSepia    PaletteIndex = 25
	PaletteRedHot   PaletteIndex = 26
)

// SetPalette sets the active color palette by index.
func (d *Device) SetPalette(index PaletteIndex) error {
	return d.setPalette(index)
}

func (d *Device) setPalette(index PaletteIndex) error {
	return d.SendCommandNoResponse(commandWithByte5(CmdSetPaletteIdx, byte(index)))
}

// GetPalette reads the currently active color palette index.
func (d *Device) GetPalette() (PaletteIndex, error) {
	resp, err := d.SendCommandWithResponse(CmdGetPaletteIdx, 1)
	if err != nil {
		return 0, fmt.Errorf("getting palette: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing palette response: %w", err)
	}

	return PaletteIndex(v), nil
}
