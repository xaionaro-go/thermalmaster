package thermalmaster

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJointBilateralUpsample_OutputDimensions(t *testing.T) {
	const (
		w = 8
		h = 6
	)
	thermal := make([]RawThermalValue, w*h)
	ir := make([]uint8, w*h)

	cfg := DefaultUpscaleConfig()
	out := JointBilateralUpsample(thermal, ir, w, h, cfg)

	require.Len(t, out, w*cfg.Factor*h*cfg.Factor)
}

func TestJointBilateralUpsample_UniformInput(t *testing.T) {
	const (
		w   = 16
		h   = 12
		val = RawThermalValue(19200) // ~300K
	)

	thermal := make([]RawThermalValue, w*h)
	ir := make([]uint8, w*h)
	for i := range thermal {
		thermal[i] = val
		ir[i] = 128
	}

	cfg := DefaultUpscaleConfig()
	out := JointBilateralUpsample(thermal, ir, w, h, cfg)

	outW := w * cfg.Factor
	outH := h * cfg.Factor
	require.Len(t, out, outW*outH)

	for i, v := range out {
		assert.Equal(t, val, v, "pixel %d should equal uniform input value", i)
	}
}

func TestJointBilateralUpsample_EdgePreservedWithIRGuide(t *testing.T) {
	// A sharp edge at column 8: left side cold, right side hot.
	// IR image has a matching edge at the same location.
	// The upscaled output should preserve this edge sharply.
	const (
		w        = 16
		h        = 12
		coldVal  = RawThermalValue(19000)
		hotVal   = RawThermalValue(20000)
		darkIR   = uint8(50)
		brightIR = uint8(200)
	)

	thermal := make([]RawThermalValue, w*h)
	ir := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			switch {
			case x < 8:
				thermal[idx] = coldVal
				ir[idx] = darkIR
			default:
				thermal[idx] = hotVal
				ir[idx] = brightIR
			}
		}
	}

	cfg := DefaultUpscaleConfig()
	out := JointBilateralUpsample(thermal, ir, w, h, cfg)

	outW := w * cfg.Factor

	// Check pixels well inside each side (away from the boundary).
	// Left side at output column 4 (source column 2):
	leftVal := out[6*outW+4]
	assert.Equal(t, coldVal, leftVal, "left side should be cold")

	// Right side at output column 28 (source column 14):
	rightVal := out[6*outW+28]
	assert.Equal(t, hotVal, rightVal, "right side should be hot")

	// At the edge boundary (output columns 15 and 16, source columns ~7.5 and ~8.0),
	// the edge should be sharp because the IR guide reinforces it.
	edgeLeft := out[6*outW+14]
	edgeRight := out[6*outW+18]
	assert.Equal(t, coldVal, edgeLeft, "edge left should stay cold with IR guide")
	assert.Equal(t, hotVal, edgeRight, "edge right should stay hot with IR guide")
}

func TestJointBilateralUpsample_EdgeSmoothedWithoutIRGuide(t *testing.T) {
	// Thermal has a sharp edge at column 8, but IR is uniform.
	// Without IR edge guidance, the thermal edge should be smoothed.
	const (
		w       = 16
		h       = 12
		coldVal = RawThermalValue(19000)
		hotVal  = RawThermalValue(20000)
	)

	thermal := make([]RawThermalValue, w*h)
	ir := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if x < 8 {
				thermal[idx] = coldVal
			} else {
				thermal[idx] = hotVal
			}
			ir[idx] = 128 // uniform IR
		}
	}

	cfg := DefaultUpscaleConfig()
	out := JointBilateralUpsample(thermal, ir, w, h, cfg)

	outW := w * cfg.Factor

	// Near the edge, values should be somewhere between cold and hot
	// because uniform IR provides no edge guidance.
	edgeVal := out[6*outW+16] // output column 16 = source column ~8
	cold := float64(coldVal)
	hot := float64(hotVal)
	mid := (cold + hot) / 2

	// The edge pixel should be blended (not exactly cold or hot).
	// Allow it to be within the range but not at either extreme.
	assert.Greater(t, float64(edgeVal), cold,
		"edge pixel should be warmer than pure cold due to smoothing")
	assert.Less(t, float64(edgeVal), hot,
		"edge pixel should be cooler than pure hot due to smoothing")

	// Should be reasonably close to the midpoint.
	assert.InDelta(t, mid, float64(edgeVal), (hot-cold)*0.6,
		"edge pixel should be near midpoint when IR is uniform")
}

func TestJointBilateralUpsample_GradientWithUniformIR(t *testing.T) {
	// Thermal has a horizontal gradient, IR is uniform.
	// Output should be a smoothly interpolated gradient.
	const (
		w = 16
		h = 4
	)

	thermal := make([]RawThermalValue, w*h)
	ir := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			thermal[idx] = RawThermalValue(19000 + x*60) // linear gradient
			ir[idx] = 128
		}
	}

	cfg := DefaultUpscaleConfig()
	out := JointBilateralUpsample(thermal, ir, w, h, cfg)

	outW := w * cfg.Factor
	outH := h * cfg.Factor

	// The output should be roughly monotonically increasing along x
	// in the interior rows.
	midRow := outH / 2
	for ox := 1; ox < outW-1; ox++ {
		prev := float64(out[midRow*outW+ox-1])
		curr := float64(out[midRow*outW+ox])
		assert.GreaterOrEqual(t, curr, prev-1.0,
			"gradient should be monotonic at output column %d", ox)
	}
}

func TestJointBilateralUpsample_DefaultConfig(t *testing.T) {
	cfg := DefaultUpscaleConfig()
	assert.Equal(t, 2, cfg.Factor)
	assert.InDelta(t, 1.5, cfg.SpatialSigma, 1e-9)
	assert.InDelta(t, 25.0, cfg.RangeSigma, 1e-9)
	assert.Equal(t, 1, cfg.WindowRadius)
}

func TestBilinearInterpolateIR(t *testing.T) {
	// 2x2 image with known values.
	ir := []uint8{10, 20, 30, 40}
	w, h := 2, 2

	// Corner values should match exactly.
	assert.InDelta(t, 10.0, bilinearInterpolateIR(ir, 0, 0, w, h), 1e-9)
	assert.InDelta(t, 20.0, bilinearInterpolateIR(ir, 1, 0, w, h), 1e-9)
	assert.InDelta(t, 30.0, bilinearInterpolateIR(ir, 0, 1, w, h), 1e-9)
	assert.InDelta(t, 40.0, bilinearInterpolateIR(ir, 1, 1, w, h), 1e-9)

	// Center should be average of all four.
	center := bilinearInterpolateIR(ir, 0.5, 0.5, w, h)
	assert.InDelta(t, 25.0, center, 1e-9)
}

func TestJointBilateralUpsample_FullResolution(t *testing.T) {
	// Test with actual P3 sensor dimensions (256x192).
	const (
		w = 256
		h = 192
	)

	thermal := make([]RawThermalValue, w*h)
	ir := make([]uint8, w*h)
	for i := range thermal {
		thermal[i] = RawThermalValue(19200)
		ir[i] = 128
	}

	cfg := DefaultUpscaleConfig()
	out := JointBilateralUpsample(thermal, ir, w, h, cfg)

	require.Len(t, out, 512*384)

	// Spot-check some pixels.
	assert.Equal(t, RawThermalValue(19200), out[0])
	assert.Equal(t, RawThermalValue(19200), out[512*384-1])
}

func TestPrecomputeSpatialWeights(t *testing.T) {
	cfg := DefaultUpscaleConfig()
	weights := precomputeSpatialWeights(cfg)

	windowSize := 2*cfg.WindowRadius + 1
	require.Len(t, weights, windowSize*windowSize)

	// Center weight should be 1.0 (distance = 0).
	centerIdx := cfg.WindowRadius*windowSize + cfg.WindowRadius
	assert.InDelta(t, 1.0, weights[centerIdx], 1e-9)

	// Weights should decrease with distance from center.
	for i, w := range weights {
		assert.LessOrEqual(t, w, 1.0, "weight at index %d should be <= 1.0", i)
		assert.Greater(t, w, 0.0, "weight at index %d should be > 0.0", i)
	}

	// A corner weight should be less than an edge-center weight.
	edgeCenter := cfg.WindowRadius*windowSize + 0 // (0, center)
	corner := 0                                    // (0, 0)
	assert.Greater(t, weights[edgeCenter], weights[corner],
		"edge center should have higher weight than corner")
}

func TestJointBilateralUpsample_SinglePixel(t *testing.T) {
	thermal := []RawThermalValue{19200}
	ir := []uint8{128}

	cfg := DefaultUpscaleConfig()
	out := JointBilateralUpsample(thermal, ir, 1, 1, cfg)

	require.Len(t, out, cfg.Factor*cfg.Factor)
	for _, v := range out {
		assert.Equal(t, RawThermalValue(19200), v)
	}
}

func TestJointBilateralUpsample_Factor3(t *testing.T) {
	const (
		w      = 4
		h      = 4
		factor = 3
	)

	thermal := make([]RawThermalValue, w*h)
	ir := make([]uint8, w*h)
	for i := range thermal {
		thermal[i] = 19200
		ir[i] = 128
	}

	cfg := UpscaleConfig{
		Factor:       factor,
		SpatialSigma: 1.5,
		RangeSigma:   25.0,
		WindowRadius: 2,
	}
	out := JointBilateralUpsample(thermal, ir, w, h, cfg)
	require.Len(t, out, w*factor*h*factor)

	for _, v := range out {
		assert.Equal(t, RawThermalValue(19200), v)
	}
}

func TestBilinearInterpolateIR_OutOfBounds(t *testing.T) {
	ir := []uint8{100, 200, 150, 250}
	w, h := 2, 2

	// Negative coordinates should clamp to edge.
	val := bilinearInterpolateIR(ir, -1, -1, w, h)
	assert.InDelta(t, 100.0, val, 1e-9, "negative coords should clamp to top-left")

	// Beyond-edge coordinates should clamp.
	val = bilinearInterpolateIR(ir, float64(w), float64(h), w, h)
	assert.False(t, math.IsNaN(val), "out of bounds should not produce NaN")
}
