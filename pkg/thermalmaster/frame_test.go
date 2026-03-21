package thermalmaster

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMarker(t *testing.T) {
	data := make([]byte, MarkerSize)
	data[0] = 0x0C        // Length
	data[1] = SyncStartEven // Sync
	binary.LittleEndian.PutUint32(data[2:6], 12345)   // Cnt1
	binary.LittleEndian.PutUint32(data[6:10], 67890)   // Cnt2
	binary.LittleEndian.PutUint16(data[10:12], 400)    // Cnt3

	m := ParseMarker(data)

	assert.Equal(t, uint8(0x0C), m.Length)
	assert.Equal(t, uint8(SyncStartEven), m.Sync)
	assert.Equal(t, uint32(12345), m.Cnt1)
	assert.Equal(t, uint32(67890), m.Cnt2)
	assert.Equal(t, uint16(400), m.Cnt3)
}

// buildSyntheticFrame builds a synthetic frame buffer with start marker,
// pixel data, and end marker. The thermal rows are filled with the given
// raw temperature value.
func buildSyntheticFrame(
	t *testing.T,
	cfg ModelConfig,
	thermalRaw RawThermalValue,
	irBrightness uint8,
) []byte {
	t.Helper()

	frameReadSize := cfg.FrameReadSize()
	buf := make([]byte, frameReadSize)

	// Start marker.
	buf[0] = 0x0C
	buf[1] = SyncStartEven
	binary.LittleEndian.PutUint32(buf[2:6], 100)
	binary.LittleEndian.PutUint32(buf[6:10], 200)
	binary.LittleEndian.PutUint16(buf[10:12], 40)

	pixelData := buf[MarkerSize : frameReadSize-MarkerSize]

	// Fill IR rows (rows 0..sensor_h-1) with brightness in low byte.
	irEnd := cfg.IRRowEnd() * cfg.SensorW * 2
	for i := 0; i < irEnd; i += 2 {
		pixelData[i] = irBrightness
		pixelData[i+1] = 0x00
	}

	// Fill thermal rows (rows sensor_h+2..2*sensor_h+1) with thermalRaw.
	thermalStart := cfg.ThermalRowStart() * cfg.SensorW * 2
	thermalEnd := cfg.ThermalRowEnd() * cfg.SensorW * 2
	for i := thermalStart; i < thermalEnd; i += 2 {
		binary.LittleEndian.PutUint16(pixelData[i:i+2], uint16(thermalRaw))
	}

	// End marker.
	endMarkerOffset := frameReadSize - MarkerSize
	buf[endMarkerOffset] = 0x0C
	buf[endMarkerOffset+1] = SyncEndEven
	binary.LittleEndian.PutUint32(buf[endMarkerOffset+2:endMarkerOffset+6], 100)
	binary.LittleEndian.PutUint32(buf[endMarkerOffset+6:endMarkerOffset+10], 200)
	binary.LittleEndian.PutUint16(buf[endMarkerOffset+10:endMarkerOffset+12], 40)

	// ReadFrame returns start_marker + pixel_data (without end marker).
	return buf[:frameReadSize-MarkerSize]
}

func TestExtractThermalData(t *testing.T) {
	cfg := ConfigP3
	thermalRaw := RawThermalValue(19200) // 300K = 26.85C
	frameData := buildSyntheticFrame(t, cfg, thermalRaw, 128)

	thermal := ExtractThermalData(frameData, cfg)
	require.NotNil(t, thermal)
	assert.Equal(t, cfg.SensorW*cfg.SensorH, len(thermal))

	for i, v := range thermal {
		assert.Equal(t, thermalRaw, v, "pixel %d", i)
	}
}

func TestExtractIRBrightness(t *testing.T) {
	cfg := ConfigP3
	irBrightness := uint8(200)
	frameData := buildSyntheticFrame(t, cfg, 19200, irBrightness)

	ir := ExtractIRBrightness(frameData, cfg)
	require.NotNil(t, ir)
	assert.Equal(t, cfg.SensorW*cfg.SensorH, len(ir))

	for i, v := range ir {
		assert.Equal(t, irBrightness, v, "pixel %d", i)
	}
}

func TestExtractBoth(t *testing.T) {
	cfg := ConfigP3
	thermalRaw := RawThermalValue(19200)
	irBrightness := uint8(150)
	frameData := buildSyntheticFrame(t, cfg, thermalRaw, irBrightness)

	ir, thermal := ExtractBoth(frameData, cfg)
	require.NotNil(t, ir)
	require.NotNil(t, thermal)
	assert.Equal(t, cfg.SensorW*cfg.SensorH, len(ir))
	assert.Equal(t, cfg.SensorW*cfg.SensorH, len(thermal))

	assert.Equal(t, irBrightness, ir[0])
	assert.Equal(t, thermalRaw, thermal[0])
}

func TestExtractThermalDataShortInput(t *testing.T) {
	cfg := ConfigP3
	assert.Nil(t, ExtractThermalData(nil, cfg))
	assert.Nil(t, ExtractThermalData([]byte{0x00}, cfg))
	assert.Nil(t, ExtractThermalData(make([]byte, 10), cfg))
}

func TestExtractIRBrightnessShortInput(t *testing.T) {
	cfg := ConfigP3
	assert.Nil(t, ExtractIRBrightness(nil, cfg))
	assert.Nil(t, ExtractIRBrightness([]byte{0x00}, cfg))
	assert.Nil(t, ExtractIRBrightness(make([]byte, 10), cfg))
}

func TestExtractWithP1Config(t *testing.T) {
	cfg := ConfigP1
	thermalRaw := RawThermalValue(19200)
	irBrightness := uint8(100)
	frameData := buildSyntheticFrame(t, cfg, thermalRaw, irBrightness)

	thermal := ExtractThermalData(frameData, cfg)
	require.NotNil(t, thermal)
	assert.Equal(t, cfg.SensorW*cfg.SensorH, len(thermal))
	assert.Equal(t, thermalRaw, thermal[0])

	ir := ExtractIRBrightness(frameData, cfg)
	require.NotNil(t, ir)
	assert.Equal(t, cfg.SensorW*cfg.SensorH, len(ir))
	assert.Equal(t, irBrightness, ir[0])
}
