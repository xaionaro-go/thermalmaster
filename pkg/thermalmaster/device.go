package thermalmaster

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// USB control transfer constants.
const (
	bmRequestTypeOut    = 0x41 // OUT | VENDOR | INTERFACE
	bmRequestTypeIn     = 0xC1 // IN | VENDOR | INTERFACE
	bmRequestTypeDevOut = 0x40 // OUT | VENDOR | DEVICE
	bRequestSendCmd     = 0x20
	bRequestReadResp    = 0x21
	bRequestReadStatus  = 0x22
	bRequestStartStream = 0xEE
	// statusPollLimit is the maximum number of status reads before timing out.
	// The native usb_status_check_done uses a caller-provided limit; we use a
	// generous default that covers slow operations like gain switching.
	statusPollLimit = 1000
)

// Device represents an opened ThermalMaster camera connected via USB.
type Device struct {
	transport      USBTransport
	config         ModelConfig
	deviceType     DeviceType
	lifecycleMu    sync.Mutex
	controlMu      sync.Mutex
	stateMu        sync.Mutex
	inFlight       sync.WaitGroup
	lifecycle      deviceLifecycle
	streamingPhase StreamingInterfacePhase
	stats          FrameStats
	nextReadID     readID
	readCancels    map[readID]context.CancelCauseFunc
	nextControlID  controlID
	controlCancels map[controlID]context.CancelCauseFunc
}

type readID uint64
type controlID uint64

type deviceLifecycle uint8

const (
	deviceLifecycleOpen deviceLifecycle = iota
	deviceLifecycleStarting
	deviceLifecycleStopping
	deviceLifecycleClosing
	deviceLifecycleClosed
)

func (l deviceLifecycle) String() string {
	switch l {
	case deviceLifecycleOpen:
		return "open"
	case deviceLifecycleStarting:
		return "starting"
	case deviceLifecycleStopping:
		return "stopping"
	case deviceLifecycleClosing:
		return "closing"
	case deviceLifecycleClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// DeviceType returns the detected device type.
func (d *Device) DeviceType() DeviceType {
	return d.deviceType
}

// NewDeviceWithTransport creates a Device using the provided transport.
// This is intended for testing with mock transports.
func NewDeviceWithTransport(transport USBTransport, cfg ModelConfig) *Device {
	return &Device{
		transport: transport,
		config:    cfg,
	}
}

// allConfigs lists all known model configurations keyed by PID.
var allConfigs = map[ProductID]ModelConfig{
	ConfigP3.PID: ConfigP3,
	ConfigP1.PID: ConfigP1,
}

// List enumerates all connected ThermalMaster cameras without opening them.
func List() ([]CameraInfo, error) {
	return listCameras()
}

// Open opens a ThermalMaster camera via USB. Without options it opens the
// first camera found. Use WithSerial, WithUSBAddress, or WithUSBBus to
// select a specific device when multiple cameras are connected.
func Open(opts ...OpenOption) (_ *Device, _err error) {
	var oc openConfig
	for _, o := range opts {
		o.applyOpenOption(&oc)
	}

	return openDevice(oc)
}

// Close releases all USB resources held by the device.
func (d *Device) Close() error {
	d.lifecycleMu.Lock()
	defer d.lifecycleMu.Unlock()

	d.stateMu.Lock()
	if d.lifecycle == deviceLifecycleClosed {
		d.stateMu.Unlock()
		return nil
	}
	d.lifecycle = deviceLifecycleClosing
	cancels := d.lifecycleCancelSnapshotLocked()
	d.stateMu.Unlock()

	for _, cancel := range cancels {
		cancel(errStreamingStopped)
	}
	d.inFlight.Wait()

	d.controlMu.Lock()
	defer d.controlMu.Unlock()
	restoreErr := d.restoreStreaming()
	d.stateMu.Lock()
	streamingIdle := d.streamingPhase == StreamingInterfaceIdle
	d.stateMu.Unlock()
	if restoreErr != nil && !streamingIdle {
		return fmt.Errorf("restoring streaming interface: %w", restoreErr)
	}

	closeErr := d.transport.Close()
	if closeErr == nil || isTerminalUSBResourceError(closeErr) {
		d.stateMu.Lock()
		d.lifecycle = deviceLifecycleClosed
		d.stateMu.Unlock()
	}
	if closeErr != nil {
		return errors.Join(
			wrapError("restoring streaming interface", restoreErr),
			fmt.Errorf("closing USB transport: %w", closeErr),
		)
	}
	return wrapError("restoring streaming interface", restoreErr)
}

// Config returns the model configuration for this device.
func (d *Device) Config() ModelConfig {
	return d.config
}

// Stats returns a snapshot of frame statistics.
func (d *Device) Stats() FrameStats {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	return d.stats
}

func (d *Device) readCancelSnapshotLocked() []context.CancelCauseFunc {
	cancels := make([]context.CancelCauseFunc, 0, len(d.readCancels))
	for _, cancel := range d.readCancels {
		cancels = append(cancels, cancel)
	}
	return cancels
}

func (d *Device) lifecycleCancelSnapshotLocked() []context.CancelCauseFunc {
	cancels := d.readCancelSnapshotLocked()
	for _, cancel := range d.controlCancels {
		cancels = append(cancels, cancel)
	}
	return cancels
}

func (d *Device) beginControl(
	parent context.Context,
) (context.Context, controlID, error) {
	return d.beginControlInLifecycle(parent, deviceLifecycleOpen)
}

func (d *Device) beginControlInLifecycle(
	parent context.Context,
	allowedLifecycle deviceLifecycle,
) (context.Context, controlID, error) {
	controlCtx, id, err := d.registerControlInLifecycle(parent, allowedLifecycle)
	if err != nil {
		return nil, 0, err
	}

	d.controlMu.Lock()
	if cause := context.Cause(controlCtx); cause != nil {
		d.endControl(id)
		return nil, 0, cause
	}
	return controlCtx, id, nil
}

func (d *Device) registerControlInLifecycle(
	parent context.Context,
	allowedLifecycle deviceLifecycle,
) (context.Context, controlID, error) {
	controlCtx, cancel := context.WithCancelCause(parent)
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	if d.lifecycle != allowedLifecycle {
		lifecycle := d.lifecycle
		err := fmt.Errorf("device is %s", lifecycle)
		cancel(err)
		return nil, 0, err
	}
	id := d.registerControlLocked(cancel)
	return controlCtx, id, nil
}

func (d *Device) registerControlLocked(cancel context.CancelCauseFunc) controlID {
	if d.controlCancels == nil {
		d.controlCancels = make(map[controlID]context.CancelCauseFunc)
	}
	id := d.nextControlID
	d.nextControlID++
	d.controlCancels[id] = cancel
	d.inFlight.Add(1)
	return id
}

func (d *Device) endControl(id controlID) {
	d.controlMu.Unlock()
	d.stateMu.Lock()
	cancel := d.controlCancels[id]
	delete(d.controlCancels, id)
	d.stateMu.Unlock()
	cancel(nil)
	d.inFlight.Done()
}

// sendCommand sends an 18-byte command via USB control transfer.
func (d *Device) sendCommand(ctx context.Context, cmd [CommandSize]byte) error {
	n, err := d.control(ctx, bmRequestTypeOut, bRequestSendCmd, 0, 0, cmd[:])
	if err != nil {
		return err
	}
	if n != len(cmd) {
		return fmt.Errorf("got %d bytes, want %d", n, len(cmd))
	}
	return nil
}

// readResponse reads a response of the given length via USB control transfer.
func (d *Device) readResponse(ctx context.Context, length int) ([]byte, error) {
	buf := make([]byte, length)
	n, err := d.control(ctx, bmRequestTypeIn, bRequestReadResp, 0, 0, buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// readStatus polls the status register until the camera is ready.
// Matches the usb_status_check_done loop in the vendor's USB transport:
// status=1 means busy (retry after 1ms), status>=2 means done.
func (d *Device) readStatus(ctx context.Context) (byte, error) {
	buf := make([]byte, 1)
	for range statusPollLimit {
		n, err := d.control(ctx, bmRequestTypeIn, bRequestReadStatus, 0, 0, buf)
		if err != nil {
			return 0, err
		}
		if n != len(buf) {
			return 0, fmt.Errorf("got %d bytes, want %d", n, len(buf))
		}

		if buf[0] != 1 {
			return buf[0], nil
		}

		// Status 1 = busy. Wait 1ms before polling again, matching the
		// vendor's usleep(1000) in usb_status_check_done.
		timer := time.NewTimer(time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return 0, context.Cause(ctx)
		case <-timer.C:
		}
	}
	return buf[0], fmt.Errorf("status poll timeout (stuck at busy)")
}

func (d *Device) control(
	ctx context.Context,
	requestType, request uint8,
	value, index uint16,
	data []byte,
) (int, error) {
	if cause := context.Cause(ctx); cause != nil {
		return 0, cause
	}
	n, err := d.transport.Control(requestType, request, value, index, data)
	if cause := context.Cause(ctx); cause != nil {
		return n, errors.Join(err, cause)
	}
	return n, err
}

func wrapError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func isTerminalUSBResourceError(err error) bool {
	return errors.Is(err, ErrUSBNoDevice) || errors.Is(err, ErrUSBNotFound)
}

// SendCommandWithResponse sends a command, reads status, reads response, reads
// status again. This is the standard read pattern for the P3 protocol.
func (d *Device) SendCommandWithResponse(
	cmd [CommandSize]byte,
	respLen int,
) ([]byte, error) {
	ctx, controlID, err := d.beginControl(context.Background())
	if err != nil {
		return nil, err
	}
	defer d.endControl(controlID)

	if err := d.sendCommand(ctx, cmd); err != nil {
		return nil, fmt.Errorf("sending command: %w", err)
	}
	if _, err := d.readStatus(ctx); err != nil {
		return nil, fmt.Errorf("reading status after command: %w", err)
	}

	resp, err := d.readResponse(ctx, respLen)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if _, err := d.readStatus(ctx); err != nil {
		return nil, fmt.Errorf("reading status after response: %w", err)
	}
	return resp, nil
}

// SendCommandNoResponse sends a command and reads status only (no response data).
func (d *Device) SendCommandNoResponse(cmd [CommandSize]byte) error {
	ctx, controlID, err := d.beginControl(context.Background())
	if err != nil {
		return err
	}
	defer d.endControl(controlID)

	if err := d.sendCommand(ctx, cmd); err != nil {
		return fmt.Errorf("sending command: %w", err)
	}
	if _, err := d.readStatus(ctx); err != nil {
		return fmt.Errorf("reading status: %w", err)
	}
	return nil
}
