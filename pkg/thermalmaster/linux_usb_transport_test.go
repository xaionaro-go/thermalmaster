//go:build linux

package thermalmaster

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xaionaro-go/usb"
)

func TestLinuxUSBTransportAdvertisesStreamingCapability(t *testing.T) {
	var transport USBTransport = &linuxUSBTransport{
		goUSBTransport: newGoUSBTransport(&recordingGoUSBOperations{}, false),
	}

	_, supportsStreaming := transport.(USBStreamingTransport)

	assert.True(t, supportsStreaming)
}

func TestLinuxUSBTransportRefusesCloseBeforeStreamingRestoration(t *testing.T) {
	base := newGoUSBTransport(&recordingGoUSBOperations{}, false)
	base.streamingPhase = StreamingInterfaceActive
	transport := &linuxUSBTransport{goUSBTransport: base}

	err := transport.Close()

	assert.ErrorContains(t, err, "streaming interface in active phase")
	assert.False(t, base.closed)
}

func TestLinuxUSBTransportDoesNotRetainTerminalFileAfterCloseError(t *testing.T) {
	deviceFile, err := os.CreateTemp(t.TempDir(), "usb-device")
	assert.NoError(t, err)
	assert.NoError(t, deviceFile.Close())
	transport := &linuxUSBTransport{
		goUSBTransport: newGoUSBTransport(&recordingGoUSBOperations{}, false),
		deviceFile:     deviceFile,
	}

	err = transport.Close()

	assert.ErrorContains(t, err, "closing usbfs device")
	assert.Nil(t, transport.deviceFile)
	assert.NoError(t, transport.Close())
}

func TestLinuxUSBTransportClosesDeviceFileAfterTerminalControlRelease(t *testing.T) {
	deviceFile, err := os.CreateTemp(t.TempDir(), "usb-device")
	assert.NoError(t, err)
	operations := &recordingGoUSBOperations{
		releaseErrs: []error{&LibUSBError{
			Operation: "releasing control interface",
			Code:      LibUSBErrorNoDevice,
		}},
	}
	transport := &linuxUSBTransport{
		goUSBTransport: newGoUSBTransport(operations, true),
		deviceFile:     deviceFile,
	}

	err = transport.Close()

	assert.ErrorIs(t, err, ErrUSBNoDevice)
	assert.True(t, transport.closed)
	assert.Nil(t, transport.deviceFile)
	assert.Equal(t, 1, operations.closeCalls)
	_, statErr := deviceFile.Stat()
	assert.True(t, errors.Is(statErr, os.ErrClosed))
}

func TestLinuxProductionLifecycleThroughNativeFixture(t *testing.T) {
	logPath := nativeFixtureLog(t)
	path := filepath.Join(t.TempDir(), "usb-device")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	transport, err := openLinuxUSBTransport(path)
	require.NoError(t, err)
	file := transport.deviceFile
	operations := transport.operations.(*goUSBOperations)
	device := &Device{transport: transport, config: ConfigP3}
	require.NoError(t, device.StartStreaming(context.Background()))
	frame, err := device.ReadFrame(context.Background())
	require.NoError(t, err)
	assert.Len(t, frame, MarkerSize+ConfigP3.FrameSize())
	require.NoError(t, device.Close())
	assert.Nil(t, operations.config)
	assert.Nil(t, operations.device)
	assert.Nil(t, operations.context)
	_, err = file.Stat()
	assert.ErrorIs(t, err, os.ErrClosed)
	events := nativeFixtureEvents(t, logPath)
	assert.NotContains(t, events, "device-list")
	assert.NotContains(t, events, "device-unref")
	assert.NotContains(t, events, "handle-open")
	assert.NotContains(t, events, "context-init-discovery")
	assert.NotContains(t, events, "set-configuration")
	assertNativeEventsInOrder(t, events, []string{
		"context-init-no-discovery", "handle-wrap", "descriptor", "config-descriptor", "config-descriptor-free",
		"get-configuration", "auto-detach-enable", "claim-control", "vendor-control", "claim-stream", "stream-alt1",
		"transfer-allocate", "transfer-submit", "transfer-callback", "transfer-free", "vendor-control",
		"stream-alt0", "release-stream", "release-control", "file-open-at-handle-close", "handle-close",
		"file-open-at-context-exit", "context-exit",
	})
}

func TestLinuxSetupFailuresThroughNativeFixture(t *testing.T) {
	for _, stage := range []string{"init", "wrap", "descriptor", "port-numbers", "config-descriptor", "get-configuration", "set-configuration", "auto-detach-enable", "claim-control", "control-alt release-control"} {
		t.Run(stage, func(t *testing.T) {
			logPath := nativeFixtureLog(t)
			t.Setenv("THERMALMASTER_NATIVE_FIXTURE_FAILURE", stage)
			t.Setenv("THERMALMASTER_NATIVE_FIXTURE_CONTROL_ALTERNATES", "1")
			if stage == "set-configuration" {
				t.Setenv("THERMALMASTER_NATIVE_FIXTURE_UNCONFIGURED", "1")
			}
			path := filepath.Join(t.TempDir(), "usb-device")
			require.NoError(t, os.WriteFile(path, nil, 0o600))
			transport, err := openLinuxUSBTransport(path)
			assert.Nil(t, transport)
			assert.ErrorIs(t, err, usb.ErrorBusy)
			if stage == "control-alt release-control" {
				assert.ErrorIs(t, err, usb.ErrorPipe)
			}
			events := nativeFixtureEvents(t, logPath)
			assert.NotContains(t, events, "device-list")
			assert.NotContains(t, events, "claim-stream")
			if stage != "init" {
				assert.Equal(t, 1, countNativeEvents(events, "context-exit"))
			}
			if stage != "init" && stage != "wrap" {
				assert.Equal(t, 1, countNativeEvents(events, "handle-close"))
				assertNativeEventsInOrder(t, events, []string{"handle-close", "file-open-at-context-exit", "context-exit"})
			}
			if stage == "control-alt release-control" {
				assert.Equal(t, 1, countNativeEvents(events, "release-control"))
				assertNativeEventsInOrder(t, events, []string{"control-alt", "release-control", "handle-close", "context-exit"})
			}
			entries, err := os.ReadDir("/proc/self/fd")
			require.NoError(t, err)
			for _, entry := range entries {
				target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				require.NoError(t, err)
				assert.NotEqual(t, path, target, "usbfs file leaked after setup failure")
			}
		})
	}
}

func TestLinuxCleanupRetriesThroughNativeFixture(t *testing.T) {
	for _, stage := range []string{"claim-stream", "stream-alt1", "stream-alt0", "release-stream", "release-control"} {
		t.Run(stage, func(t *testing.T) {
			logPath := nativeFixtureLog(t)
			path := filepath.Join(t.TempDir(), "usb-device")
			require.NoError(t, os.WriteFile(path, nil, 0o600))
			transport, err := openLinuxUSBTransport(path)
			require.NoError(t, err)
			file := transport.deviceFile
			if stage == "claim-stream" || stage == "stream-alt1" {
				t.Setenv("THERMALMASTER_NATIVE_FIXTURE_FAILURE", stage)
			}
			phase, activateErr := transport.ActivateStreamingInterface()
			switch stage {
			case "claim-stream":
				assert.ErrorIs(t, activateErr, usb.ErrorBusy)
				assert.Equal(t, StreamingInterfaceIdle, phase)
			case "stream-alt1":
				assert.ErrorIs(t, activateErr, usb.ErrorBusy)
				assert.Equal(t, StreamingInterfaceRestorePending, phase)
			default:
				require.NoError(t, activateErr)
			}
			t.Setenv("THERMALMASTER_NATIVE_FIXTURE_FAILURE", stage)
			device := &Device{transport: transport, config: ConfigP3, streamingPhase: phase}
			closeErr := device.Close()
			if stage == "claim-stream" || stage == "stream-alt1" {
				require.NoError(t, closeErr)
			} else {
				assert.ErrorIs(t, closeErr, usb.ErrorBusy)
				assert.False(t, transport.closed)
				_, err := file.Stat()
				assert.NoError(t, err)
				assert.NotContains(t, nativeFixtureEvents(t, logPath), "handle-close")
				t.Setenv("THERMALMASTER_NATIVE_FIXTURE_FAILURE", "")
				require.NoError(t, device.Close())
			}
			events := nativeFixtureEvents(t, logPath)
			assert.Equal(t, 1, countNativeEvents(events, "handle-close"))
			assert.Equal(t, 1, countNativeEvents(events, "context-exit"))
			if stage == "release-stream" {
				assert.Equal(t, 1, countNativeEvents(events, "stream-alt0"))
				assert.Equal(t, 2, countNativeEvents(events, "release-stream"))
			}
			_, err = file.Stat()
			assert.ErrorIs(t, err, os.ErrClosed)
		})
	}
}

func TestLinuxSelectsConfigurationThroughNativeFixture(t *testing.T) {
	logPath := nativeFixtureLog(t)
	t.Setenv("THERMALMASTER_NATIVE_FIXTURE_UNCONFIGURED", "1")
	path := filepath.Join(t.TempDir(), "usb-device")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	transport, err := openLinuxUSBTransport(path)
	require.NoError(t, err)
	require.NoError(t, transport.Close())
	events := nativeFixtureEvents(t, logPath)
	assertNativeEventsInOrder(t, events, []string{"get-configuration", "set-configuration", "auto-detach-enable", "claim-control", "release-control", "handle-close", "context-exit"})
	assert.Equal(t, 1, countNativeEvents(events, "set-configuration"))
}
