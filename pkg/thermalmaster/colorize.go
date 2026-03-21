package thermalmaster

import (
	"github.com/xaionaro-go/thermalmaster/pkg/colormap"
)

// ColorizeThermal normalizes thermal data and applies a colormap, producing
// RGB24 bytes (3 bytes per pixel). It also returns the min and max thermal
// values found, so callers can use them for legend rendering without
// re-scanning.
func ColorizeThermal(
	thermal []RawThermalValue,
	cm colormap.Colormap,
) ([]byte, RawThermalValue, RawThermalValue) {
	if len(thermal) == 0 {
		return nil, 0, 0
	}

	min, max := ThermalMinMax(thermal)

	spread := float64(max - min)
	if spread == 0 {
		spread = 1
	}

	out := make([]byte, len(thermal)*3)
	for i, v := range thermal {
		norm := float64(v-min) / spread
		c := cm.At(norm)
		out[i*3] = c.R
		out[i*3+1] = c.G
		out[i*3+2] = c.B
	}
	return out, min, max
}

// ColorizeUint8 normalizes uint8 data and applies a colormap, producing
// RGB24 bytes (3 bytes per pixel).
func ColorizeUint8(
	data []uint8,
	cm colormap.Colormap,
) []byte {
	if len(data) == 0 {
		return nil
	}

	var min, max uint8 = data[0], data[0]
	for _, v := range data[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	spread := float64(max - min)
	if spread == 0 {
		spread = 1
	}

	out := make([]byte, len(data)*3)
	for i, v := range data {
		norm := float64(v-min) / spread
		c := cm.At(norm)
		out[i*3] = c.R
		out[i*3+1] = c.G
		out[i*3+2] = c.B
	}
	return out
}
