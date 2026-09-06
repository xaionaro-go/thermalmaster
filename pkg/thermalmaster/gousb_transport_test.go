package thermalmaster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xaionaro-go/usb"
)

func TestGoUSBTransportBulkReadPreservesCancellationAndPartialBytes(t *testing.T) {
	cause := errors.New("stop bulk read")
	ctx, cancel := context.WithCancelCause(context.Background())
	calls := 0
	operations := &recordingGoUSBOperations{
		bulkReadFunc: func(readCtx context.Context, _ uint8, _ []byte) (int, error) {
			calls++
			assert.Same(t, ctx, readCtx)
			cancel(cause)
			return 3, usb.TransferCancelled
		},
	}
	transport := newGoUSBTransport(operations, false)

	n, err := transport.BulkRead(ctx, bulkEndpointAddr, make([]byte, 8))

	assert.Equal(t, 3, n)
	assert.ErrorIs(t, err, cause)
	assert.Equal(t, 1, calls)
}

func TestGoUSBTransportBulkReadChecksCancellationBeforeSubmission(t *testing.T) {
	cause := errors.New("already canceled")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)
	called := false
	operations := &recordingGoUSBOperations{
		bulkReadFunc: func(context.Context, uint8, []byte) (int, error) {
			called = true
			return 0, nil
		},
	}
	transport := newGoUSBTransport(operations, false)

	n, err := transport.BulkRead(ctx, bulkEndpointAddr, make([]byte, 8))

	assert.Zero(t, n)
	assert.ErrorIs(t, err, cause)
	assert.False(t, called)
}

func TestGoUSBOperationsBoundsControlTransfer(t *testing.T) {
	device := &usb.Device{}
	newGoUSBOperations(nil, device)
	assert.Equal(t, usbControlTransferTimeout, device.ControlTimeout)
}

func TestGoUSBTransportConsumesOnlyTerminalAlternateAndReleaseErrors(t *testing.T) {
	t.Run("alternate not-found stays pending", func(t *testing.T) {
		operations := &recordingGoUSBOperations{
			setAlternateErr: &LibUSBError{Operation: "alternate", Code: LibUSBErrorNotFound},
		}
		transport := newGoUSBTransport(operations, true)
		transport.streamingPhase = StreamingInterfaceActive

		err := transport.SetStreamingInterfaceIdle()

		assert.ErrorIs(t, err, ErrUSBNotFound)
		assert.Equal(t, StreamingInterfaceActive, transport.streamingPhase)
		assert.Zero(t, operations.releaseCalls)
	})

	t.Run("alternate disconnect advances to release", func(t *testing.T) {
		operations := &recordingGoUSBOperations{
			setAlternateErr: &LibUSBError{Operation: "alternate", Code: LibUSBErrorNoDevice},
		}
		transport := newGoUSBTransport(operations, true)
		transport.streamingPhase = StreamingInterfaceActive

		err := transport.SetStreamingInterfaceIdle()

		assert.ErrorIs(t, err, ErrUSBNoDevice)
		assert.Equal(t, StreamingInterfaceReleasePending, transport.streamingPhase)
		assert.Zero(t, operations.releaseCalls)
	})

	for _, code := range []LibUSBErrorCode{LibUSBErrorNoDevice, LibUSBErrorNotFound} {
		t.Run("terminal release", func(t *testing.T) {
			operations := &recordingGoUSBOperations{
				releaseErrs: []error{&LibUSBError{Operation: "release", Code: code}},
			}
			transport := newGoUSBTransport(operations, true)
			transport.streamingPhase = StreamingInterfaceReleasePending

			err := transport.ReleaseStreamingInterface()

			assert.Error(t, err)
			assert.Equal(t, StreamingInterfaceIdle, transport.streamingPhase)
			assert.Equal(t, 1, operations.releaseCalls)
		})
	}

	t.Run("transient release remains retryable", func(t *testing.T) {
		transientErr := errors.New("release busy")
		operations := &recordingGoUSBOperations{releaseErrs: []error{transientErr, nil}}
		transport := newGoUSBTransport(operations, true)
		transport.streamingPhase = StreamingInterfaceReleasePending

		firstErr := transport.ReleaseStreamingInterface()
		secondErr := transport.ReleaseStreamingInterface()

		assert.ErrorIs(t, firstErr, transientErr)
		assert.NoError(t, secondErr)
		assert.Equal(t, StreamingInterfaceIdle, transport.streamingPhase)
		assert.Equal(t, 2, operations.releaseCalls)
	})
}

func TestGoUSBTransportCloseConsumesTerminalControlReleaseAndClosesLocalsOnce(t *testing.T) {
	for _, code := range []LibUSBErrorCode{LibUSBErrorNoDevice, LibUSBErrorNotFound} {
		t.Run("terminal control release", func(t *testing.T) {
			operations := &recordingGoUSBOperations{
				releaseErrs: []error{&LibUSBError{Operation: "release control", Code: code}},
			}
			transport := newGoUSBTransport(operations, true)

			firstErr := transport.Close()
			secondErr := transport.Close()

			assert.Error(t, firstErr)
			assert.NoError(t, secondErr)
			assert.False(t, transport.controlClaimed)
			assert.True(t, transport.closed)
			assert.Equal(t, 1, operations.releaseCalls)
			assert.Equal(t, 1, operations.closeCalls)
		})
	}
}

func TestGoUSBTransportClosePreservesTransientControlReleaseForRetry(t *testing.T) {
	transientErr := errors.New("control release busy")
	operations := &recordingGoUSBOperations{releaseErrs: []error{transientErr, nil}}
	transport := newGoUSBTransport(operations, true)

	firstErr := transport.Close()
	secondErr := transport.Close()

	assert.ErrorIs(t, firstErr, transientErr)
	assert.NoError(t, secondErr)
	assert.Equal(t, 2, operations.releaseCalls)
	assert.Equal(t, 1, operations.closeCalls)
}

func TestPrepareGoUSBDeviceDisposesControlAfterSetupFailure(t *testing.T) {
	setupErr := errors.New("alternate failed")
	disposeErr := errors.New("release busy")
	operations := &recordingGoUSBOperations{
		configDetailsResult: goUSBConfigDetails{controlAlternateCount: 2, streamEndpoint: true},
		setAlternateErr:     setupErr,
		disposeErr:          disposeErr,
	}
	transport, err := prepareGoUSBDevice(operations)
	assert.Nil(t, transport)
	assert.ErrorIs(t, err, setupErr)
	assert.ErrorIs(t, err, disposeErr)
	assert.Equal(t, []string{"configure", "auto-detach", "claim:0", "dispose:0"}, operations.events)
	assert.Zero(t, operations.releaseCalls)
}

func TestGoUSBTransportCloseRetriesLocalOwners(t *testing.T) {
	closeErr := errors.New("config still has an interface")
	operations := &recordingGoUSBOperations{closeErr: closeErr}
	transport := newGoUSBTransport(operations, true)
	assert.ErrorIs(t, transport.Close(), closeErr)
	assert.False(t, transport.closed)
	assert.False(t, transport.controlClaimed)
	operations.closeErr = nil
	assert.NoError(t, transport.Close())
	assert.Equal(t, 1, operations.releaseCalls)
	assert.Equal(t, 2, operations.closeCalls)
}

type recordingGoUSBOperations struct {
	closeErr, disposeErr error
	releaseErrs          []error
	setAlternateErr      error
	bulkReadFunc         func(context.Context, uint8, []byte) (int, error)
	configDetailsResult  goUSBConfigDetails
	configDetailsErr     error
	autoDetachErr        error
	claimErr             error
	releaseCalls         int
	closeCalls           int
	events               []string
}

func (o *recordingGoUSBOperations) control(
	_ uint8,
	_ uint8,
	_ uint16,
	_ uint16,
	data []byte,
) (int, error) {
	return len(data), nil
}

func (o *recordingGoUSBOperations) bulkRead(
	ctx context.Context,
	endpoint uint8,
	buf []byte,
) (int, error) {
	if o.bulkReadFunc != nil {
		return o.bulkReadFunc(ctx, endpoint, buf)
	}
	return 0, nil
}

func (o *recordingGoUSBOperations) claimInterface(interfaceNumber int) error {
	o.events = append(o.events, fmt.Sprintf("claim:%d", interfaceNumber))
	return o.claimErr
}

func (o *recordingGoUSBOperations) setInterfaceAlternate(int, int) error {
	return o.setAlternateErr
}

func (o *recordingGoUSBOperations) releaseInterface(interfaceNumber int) error {
	o.events = append(o.events, fmt.Sprintf("release:%d", interfaceNumber))
	o.releaseCalls++
	if len(o.releaseErrs) == 0 {
		return nil
	}
	err := o.releaseErrs[0]
	o.releaseErrs = o.releaseErrs[1:]
	return err
}

func (o *recordingGoUSBOperations) close() error {
	o.events = append(o.events, "close")
	o.closeCalls++
	return o.closeErr
}

func (o *recordingGoUSBOperations) configure() (goUSBConfigDetails, error) {
	o.events = append(o.events, "configure")
	return o.configDetailsResult, o.configDetailsErr
}

func (o *recordingGoUSBOperations) setAutoDetach() error {
	o.events = append(o.events, "auto-detach")
	return o.autoDetachErr
}

var _ USBStreamingTransport = (*goUSBTransport)(nil)

func (o *recordingGoUSBOperations) disposeInterface(number int) error {
	o.events = append(o.events, fmt.Sprintf("dispose:%d", number))
	return o.disposeErr
}

func TestGoUSBErrorPreservesNativeAndPortableCauses(t *testing.T) {
	for _, native := range []usb.Error{usb.ErrorNoDevice, usb.ErrorNotFound, usb.ErrorBusy, usb.ErrorTimeout} {
		err := adaptGoUSBError("test operation", fmt.Errorf("wrapped: %w", native))
		var gotNative usb.Error
		var gotPortable *LibUSBError
		require.ErrorAs(t, err, &gotNative)
		require.ErrorAs(t, err, &gotPortable)
		assert.Equal(t, native, gotNative)
		assert.Equal(t, LibUSBErrorCode(native), gotPortable.Code)
		assert.Equal(t, native == usb.ErrorNoDevice, errors.Is(err, ErrUSBNoDevice))
		assert.Equal(t, native == usb.ErrorNotFound, errors.Is(err, ErrUSBNotFound))
	}
}

func TestGoUSBErrorKeepsTransferStatusesDistinct(t *testing.T) {
	for _, status := range []usb.TransferStatus{usb.TransferNoDevice, usb.TransferCancelled, usb.TransferTimedOut, usb.TransferError, usb.TransferStatus(255)} {
		err := adaptGoUSBError("read", status)
		var gotStatus usb.TransferStatus
		var native usb.Error
		var portable *LibUSBError
		require.ErrorAs(t, err, &gotStatus)
		assert.Equal(t, status, gotStatus)
		assert.False(t, errors.As(err, &native))
		assert.False(t, errors.As(err, &portable))
		assert.Equal(t, status == usb.TransferNoDevice, errors.Is(err, ErrUSBNoDevice))
	}
	unknown := errors.New("LIBUSB_ERROR_NO_DEVICE is only text here")
	err := adaptGoUSBError("read", unknown)
	assert.ErrorIs(t, err, unknown)
	assert.NotErrorIs(t, err, ErrUSBNoDevice)
	assert.NoError(t, adaptGoUSBError("read", nil))
}

func TestGoUSBCancellationThroughNativeFixture(t *testing.T) {
	logPath := nativeFixtureLog(t)
	t.Setenv("THERMALMASTER_NATIVE_FIXTURE_BLOCK", "1")
	var (
		usbContext                             *usb.Context
		operations                             *goUSBOperations
		device                                 *Device
		file, notification, notificationWriter *os.File
		readerDone                             chan struct{}
	)
	cause := errors.New("cancel submitted USB read")
	ctx, cancel := context.WithCancelCause(context.Background())
	// Register cleanup before starting the worker. A broken native cancellation
	// must fail the test process without releasing resources under a live read.
	t.Cleanup(func() {
		cancel(cause)
		if readerDone != nil {
			select {
			case <-readerDone:
			case <-time.After(5 * time.Second):
				panic("USB read did not complete after cancellation; refusing concurrent teardown")
			}
		}
		for _, pipe := range []*os.File{notification, notificationWriter} {
			if pipe != nil {
				assert.NoError(t, pipe.Close())
			}
		}
		switch {
		case device != nil:
			assert.NoError(t, device.Close())
		case operations != nil:
			assert.NoError(t, operations.close())
		case usbContext != nil:
			assert.NoError(t, usbContext.Close())
		}
		if file != nil {
			assert.NoError(t, file.Close())
		}
	})

	var err error
	file, err = os.CreateTemp(t.TempDir(), "usb-device")
	require.NoError(t, err)
	usbContext, err = usb.NewContextWithOptions(usb.ContextOptions{DeviceDiscovery: usb.DisableDeviceDiscovery})
	require.NoError(t, err)
	usbDevice, err := usbContext.OpenDeviceWithFileDescriptor(file.Fd())
	require.NoError(t, err)
	operations = newGoUSBOperations(usbContext, usbDevice)
	transport, err := prepareGoUSBDevice(operations)
	require.NoError(t, err)
	phase, err := transport.ActivateStreamingInterface()
	device = &Device{transport: transport, config: ConfigP3, streamingPhase: phase}
	require.NoError(t, err)

	notification, notificationWriter, err = os.Pipe()
	require.NoError(t, err)
	t.Setenv("THERMALMASTER_NATIVE_FIXTURE_SUBMIT_FD", strconv.FormatUint(uint64(notificationWriter.Fd()), 10))
	require.NoError(t, notification.SetReadDeadline(time.Now().Add(5*time.Second)))
	buf := make([]byte, 512)
	var n int
	var readErr error
	readerDone = make(chan struct{})
	go func() {
		n, readErr = transport.BulkRead(ctx, bulkEndpointAddr, buf)
		close(readerDone)
	}()

	var submitted [1]byte
	_, err = io.ReadFull(notification, submitted[:])
	require.NoError(t, err)
	require.Equal(t, byte('x'), submitted[0])
	cancel(cause)
	select {
	case <-readerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("submitted read did not finish after cancellation")
	}
	assert.Equal(t, 3, n)
	assert.Equal(t, "abc", string(buf[:n]))
	assert.ErrorIs(t, readErr, cause)
	assert.ErrorIs(t, readErr, usb.TransferCancelled)
	assert.NotErrorIs(t, readErr, ErrUSBNoDevice)
	events := nativeFixtureEvents(t, logPath)
	assertNativeEventsInOrder(t, events, []string{"transfer-allocate", "transfer-submit", "transfer-cancel", "transfer-callback", "transfer-free"})
	assert.NotContains(t, events, "handle-close")
	assert.Equal(t, 1, countNativeEvents(events, "transfer-submit"))
	assert.Equal(t, 1, countNativeEvents(events, "transfer-free"))
}

func TestGoUSBTransportRejectsActivationAfterClose(t *testing.T) {
	operations := &recordingGoUSBOperations{}
	transport := newGoUSBTransport(operations, false)
	require.NoError(t, transport.Close())
	require.NotPanics(t, func() {
		phase, err := transport.ActivateStreamingInterface()
		assert.ErrorContains(t, err, "closed")
		assert.Equal(t, StreamingInterfaceIdle, phase)
	})
	assert.Equal(t, []string{"close"}, operations.events)
}
