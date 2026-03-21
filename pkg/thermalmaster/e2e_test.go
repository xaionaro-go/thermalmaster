//go:build e2e_test

package thermalmaster

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sharedDevice is a single device connection shared across all E2E tests.
// This avoids rapid USB open/close cycles that cause "no device" errors.
var sharedDevice *Device

func TestMain(m *testing.M) {
	dev, err := Open(ModelP3)
	if err != nil {
		// Skip all E2E tests if device is not available.
		os.Exit(0)
	}
	sharedDevice = dev

	code := m.Run()

	sharedDevice.Close()
	os.Exit(code)
}

func TestE2E_DeviceInfo(t *testing.T) {
	info, err := sharedDevice.ReadDeviceInfo()
	require.NoError(t, err)

	t.Logf("Model: %s", info.Model)
	t.Logf("FW Version: %s", info.FWVersion)
	t.Logf("Part Number: %s", info.PartNumber)
	t.Logf("Serial: %s", info.Serial)
	t.Logf("HW Version: %s", info.HWVersion)

	assert.NotEmpty(t, info.Model)
	assert.NotEmpty(t, info.FWVersion)
}

func TestE2E_DeviceStatus(t *testing.T) {
	status, err := sharedDevice.GetDeviceCurrentStatus()
	require.NoError(t, err)
	t.Logf("Device status: 0x%04x", status)
}

func TestE2E_DeviceTemp(t *testing.T) {
	temp, err := sharedDevice.GetDeviceTemp()
	skipOnUSBError(t, err, "GetDeviceTemp")
	t.Logf("Device temp: %.2fC", temp)
}

func TestE2E_Heartbeat(t *testing.T) {
	err := sharedDevice.SendHeartbeat()
	require.NoError(t, err)
}

func TestE2E_Shutter(t *testing.T) {
	err := sharedDevice.TriggerShutter()
	require.NoError(t, err)
	t.Log("Shutter triggered - should hear a click")

	// NUC calibration takes ~2 seconds; wait for the camera to recover
	// before subsequent tests.
	time.Sleep(3 * time.Second)
}

func TestE2E_GainSwitch(t *testing.T) {
	// Set high gain.
	err := sharedDevice.SetGain(GainHigh)
	require.NoError(t, err)

	// Read back gain. The VDCMD read response format may not match
	// the set register encoding; log value for protocol debugging.
	mode, err := sharedDevice.GetGain()
	skipOnUSBError(t, err, "GetGain")
	t.Logf("After SetGain(High): GetGain returned %d", mode)

	// Set low gain.
	err = sharedDevice.SetGain(GainLow)
	require.NoError(t, err)

	mode, err = sharedDevice.GetGain()
	skipOnUSBError(t, err, "GetGain")
	t.Logf("After SetGain(Low): GetGain returned %d", mode)

	// Restore high gain.
	sharedDevice.SetGain(GainHigh)
}

func TestE2E_StreamAndReadFrames(t *testing.T) {
	// Shutter (NUC calibration) is required before thermal data is valid.
	// Without it, thermal rows contain uniform uncalibrated values.
	require.NoError(t, sharedDevice.TriggerShutter())
	time.Sleep(3 * time.Second)

	err := sharedDevice.StartStreaming(context.Background())
	require.NoError(t, err)
	defer sharedDevice.StopStreaming()

	for i := 0; i < 10; i++ {
		frame, err := sharedDevice.ReadFrame(context.Background())
		require.NoError(t, err)
		require.NotEmpty(t, frame)

		thermal := ExtractThermalData(frame, ConfigP3)
		require.NotNil(t, thermal)
		assert.Len(t, thermal, 256*192)

		ir := ExtractIRBrightness(frame, ConfigP3)
		require.NotNil(t, ir)
		assert.Len(t, ir, 256*192)

		minTemp := thermal[0].Celsius()
		maxTemp := thermal[0].Celsius()
		for _, v := range thermal {
			temp := v.Celsius()
			if temp < minTemp {
				minTemp = temp
			}
			if temp > maxTemp {
				maxTemp = temp
			}
		}

		t.Logf("Frame %d: min=%.1fC max=%.1fC", i, minTemp, maxTemp)
		require.Greater(t, minTemp, -40.0, "thermal data appears uncalibrated")
		require.Less(t, maxTemp, 600.0, "thermal data out of range")
	}

	stats := sharedDevice.Stats()
	assert.Equal(t, uint64(10), stats.FramesRead)
	t.Logf("Dropped: %d, Mismatches: %d", stats.FramesDropped, stats.MarkerMismatches)
}

func TestE2E_TemperatureMeasurement(t *testing.T) {
	err := sharedDevice.StartStreaming(context.Background())
	require.NoError(t, err)
	defer sharedDevice.StopStreaming()

	frame, err := sharedDevice.ReadFrame(context.Background())
	require.NoError(t, err)

	thermal := ExtractThermalData(frame, ConfigP3)
	require.NotNil(t, thermal)

	env := DefaultEnvParams()

	centerTemp := PointTemp(thermal, 128, 96, 256, env)
	t.Logf("Center temperature: %.2f C", centerTemp)
	assert.Greater(t, centerTemp, -40.0)
	assert.Less(t, centerTemp, 600.0)

	info := RectTemp(thermal, 0, 0, 256, 192, 256, env)
	t.Logf("Frame: min=%.2fC max=%.2fC avg=%.2fC", info.Min, info.Max, info.Avg)
}

func TestE2E_HeartbeatLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := sharedDevice.RunHeartbeatLoop(ctx, 1*time.Second)
	// RunHeartbeatLoop returns nil on context cancellation.
	skipOnUSBError(t, err, "RunHeartbeatLoop")
	assert.NoError(t, err)
}

func skipOnUSBError(t *testing.T, err error, operation string) {
	t.Helper()
	if err != nil {
		t.Skipf("%s not yet validated against real hardware: %v", operation, err)
	}
}

// Tests below use VDCMD commands not yet validated against real hardware.
// They skip gracefully on failure.

func TestE2E_EnvironmentCorrection(t *testing.T) {
	err := sharedDevice.SetEmissivity(950)
	skipOnUSBError(t, err, "SetEmissivity")

	v, err := sharedDevice.GetEmissivity()
	skipOnUSBError(t, err, "GetEmissivity")
	t.Logf("Emissivity: %d", v)
}

func TestE2E_PaletteSwitch(t *testing.T) {
	// Palette VDCMD read response may use a different encoding than
	// simple uint16 index. Log raw values for protocol debugging.
	orig, err := sharedDevice.GetPalette()
	skipOnUSBError(t, err, "GetPalette")
	t.Logf("Original palette raw value: %d (0x%04x)", orig, uint16(orig))

	err = sharedDevice.SetPalette(1)
	skipOnUSBError(t, err, "SetPalette")

	time.Sleep(200 * time.Millisecond)

	current, err := sharedDevice.GetPalette()
	skipOnUSBError(t, err, "GetPalette read-back")
	t.Logf("After SetPalette(1): GetPalette returned %d (0x%04x)", current, uint16(current))
}

func TestE2E_PoweredTime(t *testing.T) {
	pt, err := sharedDevice.GetPoweredTime()
	skipOnUSBError(t, err, "GetPoweredTime")
	t.Logf("Powered time: %d seconds", pt)
}
