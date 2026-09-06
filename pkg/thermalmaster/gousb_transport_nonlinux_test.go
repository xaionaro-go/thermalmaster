//go:build !linux

package thermalmaster

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xaionaro-go/usb"
)

func TestNonLinuxProductionLifecycleThroughNativeFixture(t *testing.T) {
	logPath := nativeFixtureLog(t)

	device, err := Open(
		WithUSBAddress(7, 9),
		WithSerial("POC123"),
	)
	require.NoError(t, err)
	require.NoError(t, device.StartStreaming(context.Background()))

	readCtx, cancelRead := context.WithTimeout(context.Background(), time.Second)
	frame, err := device.ReadFrame(readCtx)
	cancelRead()
	require.NoError(t, err)
	assert.Len(t, frame, MarkerSize+ConfigP3.FrameSize())
	require.NoError(t, device.Close())

	events := nativeFixtureEvents(t, logPath)
	assert.NotContains(t, events, "manual-detach-0")
	assert.NotContains(t, events, "manual-detach-1")
	assert.NotContains(t, events, "set-configuration")
	assertNativeEventsInOrder(t, events, []string{
		"context-init-discovery",
		"device-list",
		"device-list-free",
		"descriptor",
		"config-descriptor",
		"config-descriptor-free",
		"handle-open",
		"serial-read",
		"get-configuration",
		"auto-detach-enable",
		"claim-control",
		"vendor-control",
		"claim-stream",
		"stream-alt1",
		"transfer-submit",
		"transfer-callback",
		"transfer-free",
		"vendor-control",
		"stream-alt0",
		"release-stream",
		"release-control",
		"handle-close",
		"context-exit",
	})
}

var _ USBStreamingTransport = (*goUSBTransport)(nil)

func TestNonLinuxSelectsAccessibleCameraThroughNativeFixture(t *testing.T) {
	for _, failure := range []string{"OPEN_FIRST_FAILS", "SERIAL_FIRST_FAILS"} {
		t.Run(failure, func(t *testing.T) {
			logPath := nativeFixtureLog(t)
			t.Setenv("THERMALMASTER_NATIVE_FIXTURE_MULTIPLE", "1")
			t.Setenv("THERMALMASTER_NATIVE_FIXTURE_"+failure, "1")
			device, err := Open(WithSerial("POC123"))
			require.NoError(t, err)
			require.NotNil(t, device)
			operations := device.transport.(*goUSBTransport).operations.(*goUSBOperations)
			assert.Equal(t, 10, operations.device.Desc.Address)
			require.NoError(t, device.Close())
			events := nativeFixtureEvents(t, logPath)
			assertNativeEventsInOrder(t, events, []string{"device-list", "serial-read", "claim-control", "release-control", "handle-close", "context-exit"})
			assert.NotContains(t, events, "handle-wrap")
		})
	}
}

func TestNonLinuxSelectionFailuresThroughNativeFixture(t *testing.T) {
	for _, scenario := range []string{"address", "serial", "serial-error", "no-serial"} {
		t.Run(scenario, func(t *testing.T) {
			logPath := nativeFixtureLog(t)
			options := []OpenOption{WithSerial("missing")}
			switch scenario {
			case "address":
				options = []OpenOption{WithUSBAddress(7, 99)}
			case "serial-error":
				t.Setenv("THERMALMASTER_NATIVE_FIXTURE_FAILURE", "serial-read")
			case "no-serial":
				t.Setenv("THERMALMASTER_NATIVE_FIXTURE_NO_SERIAL", "1")
			}
			device, err := Open(options...)
			assert.Error(t, err)
			assert.Nil(t, device)
			if scenario == "serial-error" {
				assert.ErrorIs(t, err, usb.ErrorIO)
				var portable *LibUSBError
				assert.ErrorAs(t, err, &portable)
			}
			events := nativeFixtureEvents(t, logPath)
			assert.Contains(t, events, "context-exit")
			assert.NotContains(t, events, "claim-control")
			if scenario == "no-serial" {
				assert.NotContains(t, events, "serial-read")
			}
		})
	}
}
