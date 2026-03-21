package thermalmaster

// PointOverThresholdAlarm scans the thermal frame and returns every pixel whose
// corrected temperature exceeds thresholdC. The thermal slice must contain
// width*height raw sensor values.
func PointOverThresholdAlarm(
	thermal []RawThermalValue,
	width, height int,
	thresholdC float64,
	env EnvParams,
) []AlarmPoint {
	var points []AlarmPoint

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			if idx >= len(thermal) {
				return points
			}
			tempC := thermal[idx].CelsiusCorrected(env)
			if tempC > thresholdC {
				points = append(points, AlarmPoint{
					X:     x,
					Y:     y,
					TempC: tempC,
				})
			}
		}
	}
	return points
}

// RectOverThresholdAlarm checks whether any pixel within the rectangle
// (x, y, w, h) exceeds thresholdC (corrected temperature). The full-frame
// width is needed to index into the flat thermal array.
func RectOverThresholdAlarm(
	thermal []RawThermalValue,
	x, y, w, h, width int,
	thresholdC float64,
	env EnvParams,
) bool {
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			px := x + dx
			py := y + dy
			idx := py*width + px
			if idx < 0 || idx >= len(thermal) || px < 0 || px >= width {
				continue
			}
			if thermal[idx].CelsiusCorrected(env) > thresholdC {
				return true
			}
		}
	}
	return false
}
