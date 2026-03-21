package thermalmaster

import (
	"math"
	"sync"
)

// JointBilateralUpsample upscales thermal data using IR brightness as an edge guide.
//
// The algorithm produces an output at (width*factor) x (height*factor) by
// computing a weighted average of nearby source thermal pixels for each
// output pixel. The weights combine spatial proximity (gaussian on distance)
// with range similarity in the IR brightness image (gaussian on intensity
// difference), so edges visible in the sharp IR image are preserved in the
// upscaled thermal result.
//
// When cfg.NumWorkers > 0, output rows are processed in parallel across
// that many goroutines. Each goroutine writes to disjoint output rows,
// so no synchronization is needed beyond the join barrier.
func JointBilateralUpsample(
	thermal []RawThermalValue,
	ir []uint8,
	width, height int,
	cfg UpscaleConfig,
) []RawThermalValue {
	factor := cfg.Factor
	outW := width * factor
	outH := height * factor
	out := make([]RawThermalValue, outW*outH)

	params := upsampleParams{
		thermal:        thermal,
		ir:             ir,
		out:            out,
		width:          width,
		height:         height,
		outW:           outW,
		factor:         factor,
		invFactor:      1.0 / float64(factor),
		windowRadius:   cfg.WindowRadius,
		windowSize:     2*cfg.WindowRadius + 1,
		spatialWeights: precomputeSpatialWeights(cfg),
		rangeWeightLUT: precomputeRangeWeights(cfg.RangeSigma),
	}

	switch {
	case cfg.NumWorkers <= 0:
		processOutputRows(params, 0, outH)
	default:
		var wg sync.WaitGroup
		rowsPerWorker := (outH + cfg.NumWorkers - 1) / cfg.NumWorkers
		for i := 0; i < cfg.NumWorkers; i++ {
			startRow := i * rowsPerWorker
			endRow := startRow + rowsPerWorker
			if endRow > outH {
				endRow = outH
			}
			if startRow >= endRow {
				break
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				processOutputRows(params, startRow, endRow)
			}()
		}
		wg.Wait()
	}

	return out
}

// upsampleParams holds shared read-only state for the upsampling workers.
type upsampleParams struct {
	thermal        []RawThermalValue
	ir             []uint8
	out            []RawThermalValue
	width          int
	height         int
	outW           int
	factor         int
	invFactor      float64
	windowRadius   int
	windowSize     int
	spatialWeights []float64
	rangeWeightLUT [rangeWeightLUTSize]float64
}

// processOutputRows computes output pixels for rows [startRow, endRow).
// Each goroutine writes to disjoint rows so no locking is needed.
func processOutputRows(p upsampleParams, startRow, endRow int) {
	for oy := startRow; oy < endRow; oy++ {
		sy := (float64(oy)+0.5)*p.invFactor - 0.5
		syInt := int(math.Floor(sy))

		for ox := 0; ox < p.outW; ox++ {
			sx := (float64(ox)+0.5)*p.invFactor - 0.5
			sxInt := int(math.Floor(sx))

			guideIR := bilinearInterpolateIR(p.ir, sx, sy, p.width, p.height)
			guideIRInt := int(math.Round(guideIR))

			var weightSum float64
			var valueSum float64

			for dy := -p.windowRadius; dy <= p.windowRadius; dy++ {
				ny := syInt + dy
				if ny < 0 || ny >= p.height {
					continue
				}

				rowOff := ny * p.width

				for dx := -p.windowRadius; dx <= p.windowRadius; dx++ {
					nx := sxInt + dx
					if nx < 0 || nx >= p.width {
						continue
					}

					wIdx := (dy+p.windowRadius)*p.windowSize + (dx + p.windowRadius)
					ws := p.spatialWeights[wIdx]

					irDiff := guideIRInt - int(p.ir[rowOff+nx])
					if irDiff < 0 {
						irDiff = -irDiff
					}
					wr := p.rangeWeightLUT[irDiff]

					w := ws * wr
					weightSum += w
					valueSum += w * float64(p.thermal[rowOff+nx])
				}
			}

			if weightSum > 0 {
				p.out[oy*p.outW+ox] = RawThermalValue(math.Round(valueSum / weightSum))
			}
		}
	}
}

// rangeWeightLUTSize is the number of entries in the range weight lookup
// table. IR brightness values are 0-255, so the maximum absolute difference
// is 255. Index 0 means identical brightness (weight=1.0).
const rangeWeightLUTSize = 256

// precomputeRangeWeights builds a lookup table mapping integer IR brightness
// difference (0..255) to the gaussian range weight exp(-d²/(2σ²)).
func precomputeRangeWeights(rangeSigma float64) [rangeWeightLUTSize]float64 {
	var lut [rangeWeightLUTSize]float64
	denom := 2.0 * rangeSigma * rangeSigma
	for d := 0; d < rangeWeightLUTSize; d++ {
		lut[d] = math.Exp(-float64(d*d) / denom)
	}
	return lut
}

// precomputeSpatialWeights builds the gaussian spatial weight table
// for the given window radius and spatial sigma.
func precomputeSpatialWeights(cfg UpscaleConfig) []float64 {
	windowSize := 2*cfg.WindowRadius + 1
	weights := make([]float64, windowSize*windowSize)
	denom := 2.0 * cfg.SpatialSigma * cfg.SpatialSigma

	for dy := -cfg.WindowRadius; dy <= cfg.WindowRadius; dy++ {
		for dx := -cfg.WindowRadius; dx <= cfg.WindowRadius; dx++ {
			dist2 := float64(dx*dx + dy*dy)
			idx := (dy+cfg.WindowRadius)*windowSize + (dx + cfg.WindowRadius)
			weights[idx] = math.Exp(-dist2 / denom)
		}
	}

	return weights
}

// bilinearInterpolateIR returns the bilinearly interpolated IR brightness
// at fractional source coordinates (x, y).
func bilinearInterpolateIR(ir []uint8, x, y float64, width, height int) float64 {
	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	x1 := x0 + 1
	y1 := y0 + 1

	// Clamp all indices to valid range.
	x0 = clampInt(x0, 0, width-1)
	y0 = clampInt(y0, 0, height-1)
	x1 = clampInt(x1, 0, width-1)
	y1 = clampInt(y1, 0, height-1)

	fx := x - math.Floor(x)
	fy := y - math.Floor(y)

	v00 := float64(ir[y0*width+x0])
	v10 := float64(ir[y0*width+x1])
	v01 := float64(ir[y1*width+x0])
	v11 := float64(ir[y1*width+x1])

	return v00*(1-fx)*(1-fy) + v10*fx*(1-fy) + v01*(1-fx)*fy + v11*fx*fy
}

func clampInt(v, lo, hi int) int {
	switch {
	case v < lo:
		return lo
	case v > hi:
		return hi
	default:
		return v
	}
}
