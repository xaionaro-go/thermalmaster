package thermalmaster

import "math"

// PointTemp returns the corrected temperature at a specific pixel.
func PointTemp(
	thermal []RawThermalValue,
	x, y, width int,
	env EnvParams,
) float64 {
	if x < 0 || x >= width || y < 0 {
		return 0
	}
	idx := y*width + x
	if idx >= len(thermal) {
		return 0
	}
	return thermal[idx].CelsiusCorrected(env)
}

// RectTemp returns temperature statistics for a rectangular region.
func RectTemp(
	thermal []RawThermalValue,
	x, y, w, h, width int,
	env EnvParams,
) TempInfo {
	info := TempInfo{
		Min: math.MaxFloat64,
		Max: -math.MaxFloat64,
	}

	count := 0
	sum := 0.0

	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			px := x + dx
			py := y + dy
			if px < 0 || px >= width || py < 0 {
				continue
			}
			idx := py*width + px
			if idx >= len(thermal) {
				continue
			}

			temp := thermal[idx].CelsiusCorrected(env)
			sum += temp
			count++

			if temp < info.Min {
				info.Min = temp
				info.MinX = px
				info.MinY = py
			}
			if temp > info.Max {
				info.Max = temp
				info.MaxX = px
				info.MaxY = py
			}
		}
	}

	if count > 0 {
		info.Avg = sum / float64(count)
	}
	return info
}

// LineTemp returns temperature statistics along a line (Bresenham algorithm).
func LineTemp(
	thermal []RawThermalValue,
	x1, y1, x2, y2, width int,
	env EnvParams,
) TempInfo {
	info := TempInfo{
		Min: math.MaxFloat64,
		Max: -math.MaxFloat64,
	}

	count := 0
	sum := 0.0

	dx := intAbs(x2 - x1)
	dy := -intAbs(y2 - y1)

	sx := 1
	if x1 > x2 {
		sx = -1
	}
	sy := 1
	if y1 > y2 {
		sy = -1
	}

	lineErr := dx + dy
	x, y := x1, y1

	for {
		idx := y*width + x
		if x >= 0 && x < width && y >= 0 && idx >= 0 && idx < len(thermal) {
			temp := thermal[idx].CelsiusCorrected(env)
			sum += temp
			count++
			if temp < info.Min {
				info.Min = temp
				info.MinX = x
				info.MinY = y
			}
			if temp > info.Max {
				info.Max = temp
				info.MaxX = x
				info.MaxY = y
			}
		}

		if x == x2 && y == y2 {
			break
		}

		e2 := 2 * lineErr
		if e2 >= dy {
			lineErr += dy
			x += sx
		}
		if e2 <= dx {
			lineErr += dx
			y += sy
		}
	}

	if count > 0 {
		info.Avg = sum / float64(count)
	}
	return info
}

func intAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
