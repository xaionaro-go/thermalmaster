//go:build e2e_test

package thermalmaster

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type e2eSelector struct {
	Bus     int
	Address int
}

type e2eCycleDevice interface {
	StartStreaming(context.Context) error
	ReadFrame(context.Context) ([]byte, error)
	StopStreaming() error
	Close() error
}

type e2eCycleEnvironment interface {
	Open(e2eSelector) (e2eCycleDevice, error)
	RequireAltZero(e2eSelector, int, string) error
}

func runThreeReopenCycles(
	required bool,
	selector e2eSelector,
	environment e2eCycleEnvironment,
) (bool, error) {
	for cycle := 1; cycle <= 3; cycle++ {
		device, err := environment.Open(selector)
		if err != nil {
			if cycle == 1 && !required {
				return true, nil
			}
			return false, fmt.Errorf("cycle %d open: %w", cycle, err)
		}
		if err := runReopenCycle(cycle, selector, device, environment); err != nil {
			return false, err
		}
	}
	return false, nil
}

func runReopenCycle(
	cycle int,
	selector e2eSelector,
	device e2eCycleDevice,
	environment e2eCycleEnvironment,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	var cycleErrors []error
	startErr := device.StartStreaming(ctx)
	if startErr != nil {
		cycleErrors = append(cycleErrors, fmt.Errorf("cycle %d start: %w", cycle, startErr))
	} else {
		frame, readErr := device.ReadFrame(ctx)
		switch {
		case readErr != nil:
			cycleErrors = append(cycleErrors, fmt.Errorf("cycle %d read: %w", cycle, readErr))
		case len(frame) == 0:
			cycleErrors = append(cycleErrors, fmt.Errorf("cycle %d read: empty frame", cycle))
		}
		cancel()
		if err := device.StopStreaming(); err != nil {
			cycleErrors = append(cycleErrors, fmt.Errorf("cycle %d stop: %w", cycle, err))
		}
		if err := environment.RequireAltZero(selector, cycle, "stop"); err != nil {
			cycleErrors = append(cycleErrors, fmt.Errorf("cycle %d stop alternate: %w", cycle, err))
		}
	}
	cancel()
	if err := device.Close(); err != nil {
		cycleErrors = append(cycleErrors, fmt.Errorf("cycle %d close: %w", cycle, err))
	}
	if err := environment.RequireAltZero(selector, cycle, "close"); err != nil {
		cycleErrors = append(cycleErrors, fmt.Errorf("cycle %d close alternate: %w", cycle, err))
	}
	return errors.Join(cycleErrors...)
}

func parseE2ERequired(value string) (bool, error) {
	switch value {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("must be exactly true, false, or unset; got %q", value)
	}
}

func e2eRequired(t *testing.T) bool {
	t.Helper()
	required, err := parseE2ERequired(os.Getenv("THERMALMASTER_E2E_REQUIRED"))
	if err != nil {
		t.Fatalf("THERMALMASTER_E2E_REQUIRED %v", err)
	}
	return required
}

func TestE2ERequiredParsing(t *testing.T) {
	tests := []struct {
		value    string
		required bool
		wantErr  bool
	}{
		{value: ""},
		{value: "false"},
		{value: "true", required: true},
		{value: "TRUE", wantErr: true},
		{value: "1", wantErr: true},
		{value: " false ", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.value), func(t *testing.T) {
			required, err := parseE2ERequired(tt.value)
			assert.Equal(t, tt.required, required)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestRunThreeReopenCyclesFirstOpenPolicy(t *testing.T) {
	openErr := errors.New("camera unavailable")
	selector := e2eSelector{Bus: 7, Address: 19}

	optionalEnvironment := &fakeE2ECycleEnvironment{
		openErrors: map[int]error{1: openErr},
	}
	skipped, err := runThreeReopenCycles(false, selector, optionalEnvironment)
	assert.True(t, skipped)
	assert.NoError(t, err)
	assert.Equal(t, []e2eSelector{selector}, optionalEnvironment.selectors)

	requiredEnvironment := &fakeE2ECycleEnvironment{
		openErrors: map[int]error{1: openErr},
	}
	skipped, err = runThreeReopenCycles(true, selector, requiredEnvironment)
	assert.False(t, skipped)
	assert.ErrorIs(t, err, openErr)
	assert.Equal(t, []e2eSelector{selector}, requiredEnvironment.selectors)
}

func TestRunThreeReopenCyclesNeverSkipsReopenFailure(t *testing.T) {
	reopenErr := errors.New("reopen failed")
	selector := e2eSelector{Bus: 7, Address: 19}
	firstDevice := newFakeE2ECycleDevice()
	environment := &fakeE2ECycleEnvironment{
		devices:    []*fakeE2ECycleDevice{firstDevice},
		openErrors: map[int]error{2: reopenErr},
	}

	skipped, err := runThreeReopenCycles(false, selector, environment)

	assert.False(t, skipped)
	assert.ErrorIs(t, err, reopenErr)
	assert.Equal(t, []e2eSelector{selector, selector}, environment.selectors)
	assert.Equal(t, 1, firstDevice.closeCalls)
}

func TestRunThreeReopenCyclesClosesAfterStartFailureWithoutStopping(t *testing.T) {
	startErr := errors.New("start failed")
	device := newFakeE2ECycleDevice()
	device.startErr = startErr
	environment := &fakeE2ECycleEnvironment{devices: []*fakeE2ECycleDevice{device}}

	skipped, err := runThreeReopenCycles(true, e2eSelector{Bus: 7, Address: 19}, environment)

	assert.False(t, skipped)
	assert.ErrorIs(t, err, startErr)
	assert.Equal(t, 1, device.startCalls)
	assert.Zero(t, device.readCalls)
	assert.Zero(t, device.stopCalls)
	assert.Equal(t, 1, device.closeCalls)
}

func TestRunThreeReopenCyclesJoinsReadStopCloseAndAltFailures(t *testing.T) {
	readErr := errors.New("read failed")
	stopErr := errors.New("stop failed")
	closeErr := errors.New("close failed")
	stopAltErr := errors.New("stop alternate failed")
	closeAltErr := errors.New("close alternate failed")
	device := newFakeE2ECycleDevice()
	device.readErr = readErr
	device.stopErr = stopErr
	device.closeErr = closeErr
	environment := &fakeE2ECycleEnvironment{
		devices: []*fakeE2ECycleDevice{device},
		altErrors: map[string]error{
			"1:stop":  stopAltErr,
			"1:close": closeAltErr,
		},
	}

	skipped, err := runThreeReopenCycles(false, e2eSelector{Bus: 7, Address: 19}, environment)

	assert.False(t, skipped)
	assert.ErrorIs(t, err, readErr)
	assert.ErrorIs(t, err, stopErr)
	assert.ErrorIs(t, err, closeErr)
	assert.ErrorIs(t, err, stopAltErr)
	assert.ErrorIs(t, err, closeAltErr)
	assert.Equal(t, 1, device.startCalls)
	assert.Equal(t, 1, device.readCalls)
	assert.Equal(t, 1, device.stopCalls)
	assert.Equal(t, 1, device.closeCalls)
}

func TestRunThreeReopenCyclesCleanupFailuresNeverSkipInEitherMode(t *testing.T) {
	for _, required := range []bool{false, true} {
		mode := "optional"
		if required {
			mode = "required"
		}
		for _, operation := range []string{"stop", "close"} {
			t.Run(mode+"_"+operation, func(t *testing.T) {
				cleanupErr := fmt.Errorf("%s failed", operation)
				device := newFakeE2ECycleDevice()
				switch operation {
				case "stop":
					device.stopErr = cleanupErr
				case "close":
					device.closeErr = cleanupErr
				}
				environment := &fakeE2ECycleEnvironment{
					devices: []*fakeE2ECycleDevice{device},
				}

				skipped, err := runThreeReopenCycles(
					required,
					e2eSelector{Bus: 7, Address: 19},
					environment,
				)

				assert.False(t, skipped)
				assert.ErrorIs(t, err, cleanupErr)
				assert.Equal(t, 1, device.stopCalls)
				assert.Equal(t, 1, device.closeCalls)
			})
		}
	}
}

func TestRunThreeReopenCyclesOpensExactlyThreeTimesWithIdenticalSelector(t *testing.T) {
	selector := e2eSelector{Bus: 7, Address: 19}
	devices := []*fakeE2ECycleDevice{
		newFakeE2ECycleDevice(),
		newFakeE2ECycleDevice(),
		newFakeE2ECycleDevice(),
	}
	environment := &fakeE2ECycleEnvironment{devices: devices}

	skipped, err := runThreeReopenCycles(true, selector, environment)

	assert.False(t, skipped)
	assert.NoError(t, err)
	assert.Equal(t, []e2eSelector{selector, selector, selector}, environment.selectors)
	for cycle, device := range devices {
		assert.Equal(t, 1, device.startCalls, "cycle %d", cycle+1)
		assert.Equal(t, 1, device.readCalls, "cycle %d", cycle+1)
		assert.Equal(t, 1, device.stopCalls, "cycle %d", cycle+1)
		assert.Equal(t, 1, device.closeCalls, "cycle %d", cycle+1)
	}
	assert.Equal(t, []string{
		"1:stop", "1:close",
		"2:stop", "2:close",
		"3:stop", "3:close",
	}, environment.altChecks)
}

type fakeE2ECycleEnvironment struct {
	selectors  []e2eSelector
	devices    []*fakeE2ECycleDevice
	openErrors map[int]error
	altErrors  map[string]error
	altChecks  []string
}

func (e *fakeE2ECycleEnvironment) Open(selector e2eSelector) (e2eCycleDevice, error) {
	e.selectors = append(e.selectors, selector)
	call := len(e.selectors)
	if err := e.openErrors[call]; err != nil {
		return nil, err
	}
	if call > len(e.devices) {
		return nil, fmt.Errorf("unexpected open call %d", call)
	}
	return e.devices[call-1], nil
}

func (e *fakeE2ECycleEnvironment) RequireAltZero(
	_ e2eSelector,
	cycle int,
	stage string,
) error {
	key := fmt.Sprintf("%d:%s", cycle, stage)
	e.altChecks = append(e.altChecks, key)
	return e.altErrors[key]
}

type fakeE2ECycleDevice struct {
	startErr   error
	readErr    error
	stopErr    error
	closeErr   error
	frame      []byte
	startCalls int
	readCalls  int
	stopCalls  int
	closeCalls int
}

func newFakeE2ECycleDevice() *fakeE2ECycleDevice {
	return &fakeE2ECycleDevice{frame: []byte{1}}
}

func (d *fakeE2ECycleDevice) StartStreaming(context.Context) error {
	d.startCalls++
	return d.startErr
}

func (d *fakeE2ECycleDevice) ReadFrame(context.Context) ([]byte, error) {
	d.readCalls++
	return d.frame, d.readErr
}

func (d *fakeE2ECycleDevice) StopStreaming() error {
	d.stopCalls++
	return d.stopErr
}

func (d *fakeE2ECycleDevice) Close() error {
	d.closeCalls++
	return d.closeErr
}

func openE2EDevice(t *testing.T, opts ...OpenOption) *Device {
	t.Helper()
	required := e2eRequired(t)
	dev, err := Open(opts...)
	if err != nil {
		if required {
			require.NoError(t, err, "required ThermalMaster camera is unavailable")
		}
		t.Skipf("ThermalMaster camera unavailable: %v", err)
	}
	t.Cleanup(func() {
		require.NoError(t, dev.Close(), "closing opened E2E camera")
	})
	return dev
}

func TestE2E_DeviceInfo(t *testing.T) {
	dev := openE2EDevice(t)
	info, err := dev.ReadDeviceInfo()
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
	dev := openE2EDevice(t)
	status, err := dev.GetDeviceCurrentStatus()
	require.NoError(t, err)
	t.Logf("Device status: 0x%04x", status)
}

func TestE2E_DeviceTemp(t *testing.T) {
	dev := openE2EDevice(t)
	temp, err := dev.GetDeviceTemp()
	skipOnUSBError(t, err, "GetDeviceTemp")
	t.Logf("Device temp: %.2fC", temp)
}

func TestE2E_Heartbeat(t *testing.T) {
	dev := openE2EDevice(t)
	err := dev.SendHeartbeat()
	require.NoError(t, err)
}

func TestE2E_Shutter(t *testing.T) {
	dev := openE2EDevice(t)
	err := dev.TriggerShutter()
	require.NoError(t, err)
	t.Log("Shutter triggered - should hear a click")

	// NUC calibration takes ~2 seconds; wait for the camera to recover
	// before subsequent tests.
	time.Sleep(3 * time.Second)
}

func TestE2E_GainSwitch(t *testing.T) {
	dev := openE2EDevice(t)
	// Set high gain.
	err := dev.SetGain(GainHigh)
	require.NoError(t, err)

	// Read back gain. The VDCMD read response format may not match
	// the set register encoding; log value for protocol debugging.
	mode, err := dev.GetGain()
	skipOnUSBError(t, err, "GetGain")
	t.Logf("After SetGain(High): GetGain returned %d", mode)

	// Set low gain.
	err = dev.SetGain(GainLow)
	require.NoError(t, err)

	mode, err = dev.GetGain()
	skipOnUSBError(t, err, "GetGain")
	t.Logf("After SetGain(Low): GetGain returned %d", mode)

	// Restore high gain.
	err = dev.SetGain(GainHigh)
	skipOnUSBError(t, err, "restoring high gain")
}

func TestE2E_StreamAndReadFrames(t *testing.T) {
	dev := openE2EDevice(t)
	// Shutter (NUC calibration) is required before thermal data is valid.
	// Without it, thermal rows contain uniform uncalibrated values.
	require.NoError(t, dev.TriggerShutter())
	time.Sleep(3 * time.Second)

	err := dev.StartStreaming(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, dev.StopStreaming()) })

	for i := 0; i < 10; i++ {
		frame, err := dev.ReadFrame(context.Background())
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

	stats := dev.Stats()
	assert.Equal(t, uint64(10), stats.FramesRead)
	t.Logf("Dropped: %d, Mismatches: %d", stats.FramesDropped, stats.MarkerMismatches)
}

func TestE2E_TemperatureMeasurement(t *testing.T) {
	dev := openE2EDevice(t)
	err := dev.StartStreaming(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, dev.StopStreaming()) })

	frame, err := dev.ReadFrame(context.Background())
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
	dev := openE2EDevice(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := dev.RunHeartbeatLoop(ctx, 1*time.Second)
	// RunHeartbeatLoop returns nil on context cancellation.
	skipOnUSBError(t, err, "RunHeartbeatLoop")
	assert.NoError(t, err)
}

func skipOnUSBError(t *testing.T, err error, operation string) {
	t.Helper()
	if err != nil {
		if e2eRequired(t) {
			require.NoError(t, err, "%s", operation)
		}
		t.Skipf("%s not yet validated against real hardware: %v", operation, err)
	}
}

// Tests below use VDCMD commands not yet validated against real hardware.
// They skip gracefully on failure.

func TestE2E_EnvironmentCorrection(t *testing.T) {
	dev := openE2EDevice(t)
	err := dev.SetEmissivity(950)
	skipOnUSBError(t, err, "SetEmissivity")

	v, err := dev.GetEmissivity()
	skipOnUSBError(t, err, "GetEmissivity")
	t.Logf("Emissivity: %d", v)
}

func TestE2E_PaletteSwitch(t *testing.T) {
	dev := openE2EDevice(t)
	// Palette VDCMD read response may use a different encoding than
	// simple uint16 index. Log raw values for protocol debugging.
	orig, err := dev.GetPalette()
	skipOnUSBError(t, err, "GetPalette")
	t.Logf("Original palette raw value: %d (0x%04x)", orig, uint16(orig))

	err = dev.SetPalette(1)
	skipOnUSBError(t, err, "SetPalette")

	time.Sleep(200 * time.Millisecond)

	current, err := dev.GetPalette()
	skipOnUSBError(t, err, "GetPalette read-back")
	t.Logf("After SetPalette(1): GetPalette returned %d (0x%04x)", current, uint16(current))
}

func TestE2E_PoweredTime(t *testing.T) {
	dev := openE2EDevice(t)
	pt, err := dev.GetPoweredTime()
	skipOnUSBError(t, err, "GetPoweredTime")
	t.Logf("Powered time: %d seconds", pt)
}
