package thermalmaster

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Streaming protocol constants.
const (
	streamingIntf     = 1               // USB interface number for bulk streaming
	streamingAltIdle  = 0               // Alt setting: no streaming
	streamingAltStart = 1               // Alt setting: streaming active
	preInterfaceDelay = 1 * time.Second // Delay before interface configuration
	preStreamDelay    = 2 * time.Second // Delay for camera readiness after 0xEE
)

var errStreamingStopped = errors.New("streaming stopped")

// StartStreaming starts the camera video stream following the initialization
// sequence from P3_PROTOCOL.md.
func (d *Device) StartStreaming(ctx context.Context) error {
	streamingTransport, ok := d.transport.(USBStreamingTransport)
	if !ok {
		return ErrStreamingUnsupported
	}

	controlCtx, controlID, err := d.beginStreamingStart(ctx)
	if err != nil {
		return err
	}
	d.controlMu.Lock()
	defer d.endControl(controlID)
	if cause := context.Cause(controlCtx); cause != nil {
		return d.joinStartFailureWithRestoration(cause)
	}

	if err := d.completeStartStreamHandshake(controlCtx, "initial"); err != nil {
		return d.joinStartFailureWithRestoration(err)
	}

	if err := sleepCtx(controlCtx, preInterfaceDelay); err != nil {
		return d.joinStartFailureWithRestoration(fmt.Errorf(
			"waiting before streaming interface activation: %w",
			err,
		))
	}

	phase, err := streamingTransport.ActivateStreamingInterface()
	d.stateMu.Lock()
	d.streamingPhase = phase
	d.stateMu.Unlock()
	if err != nil {
		return d.joinStartFailureWithRestoration(fmt.Errorf("activating streaming interface: %w", err))
	}
	if phase != StreamingInterfaceActive {
		return d.joinStartFailureWithRestoration(fmt.Errorf(
			"activating streaming interface returned %s phase without an error",
			phase,
		))
	}

	if err := context.Cause(controlCtx); err != nil {
		return d.joinStartFailureWithRestoration(err)
	}
	n, err := d.control(controlCtx, bmRequestTypeDevOut, bRequestStartStream, 0, streamingIntf, nil)
	if err != nil {
		return d.joinStartFailureWithRestoration(fmt.Errorf("sending start stream (0xEE): %w", err))
	}
	if n != 0 {
		return d.joinStartFailureWithRestoration(fmt.Errorf(
			"sending start stream (0xEE): got %d bytes, want 0",
			n,
		))
	}

	if err := sleepCtx(controlCtx, preStreamDelay); err != nil {
		return d.joinStartFailureWithRestoration(fmt.Errorf("waiting for camera stream readiness: %w", err))
	}

	buf := make([]byte, d.config.FrameSize())
	// This read primes the streaming pipe; its data and error are not a stream
	// success signal. The checked final handshake below determines whether
	// startup completed.
	primingCtx, cancelPriming := context.WithTimeout(controlCtx, usbStartupPrimingReadTimeout)
	_, _ = streamingTransport.BulkRead(primingCtx, bulkEndpointAddr, buf)
	cancelPriming()
	if err := context.Cause(controlCtx); err != nil {
		return d.joinStartFailureWithRestoration(err)
	}

	if err := d.completeStartStreamHandshake(controlCtx, "final"); err != nil {
		return d.joinStartFailureWithRestoration(err)
	}

	return d.completeStreamingStart(controlCtx)
}

func (d *Device) beginStreamingStart(
	parent context.Context,
) (context.Context, controlID, error) {
	controlCtx, cancel := context.WithCancelCause(parent)
	d.lifecycleMu.Lock()
	defer d.lifecycleMu.Unlock()

	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	if d.lifecycle != deviceLifecycleOpen {
		lifecycle := d.lifecycle
		err := fmt.Errorf("device is %s", lifecycle)
		cancel(err)
		return nil, 0, err
	}
	switch d.streamingPhase {
	case StreamingInterfaceIdle:
	case StreamingInterfaceActive:
		err := fmt.Errorf("already streaming")
		cancel(err)
		return nil, 0, err
	case StreamingInterfaceRestorePending:
		err := fmt.Errorf("cannot start streaming while interface restoration is pending")
		cancel(err)
		return nil, 0, err
	case StreamingInterfaceReleasePending:
		err := fmt.Errorf("cannot start streaming while interface release is pending")
		cancel(err)
		return nil, 0, err
	default:
		phase := d.streamingPhase
		err := fmt.Errorf("cannot start streaming from unknown interface phase %d", phase)
		cancel(err)
		return nil, 0, err
	}
	if err := context.Cause(controlCtx); err != nil {
		cancel(err)
		return nil, 0, err
	}

	d.lifecycle = deviceLifecycleStarting
	d.stats = FrameStats{}
	controlID := d.registerControlLocked(cancel)
	return controlCtx, controlID, nil
}

func (d *Device) completeStartStreamHandshake(ctx context.Context, stage string) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if err := d.sendCommand(ctx, CmdStartStream); err != nil {
		return fmt.Errorf("%s start_stream command: %w", stage, err)
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if _, err := d.readStatus(ctx); err != nil {
		return fmt.Errorf("%s start_stream status after command: %w", stage, err)
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}

	response, err := d.readResponse(ctx, 1)
	if err != nil {
		return fmt.Errorf("%s start_stream response: %w", stage, err)
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if len(response) != 1 {
		return fmt.Errorf("%s start_stream response: got %d bytes, want 1", stage, len(response))
	}

	if _, err := d.readStatus(ctx); err != nil {
		return fmt.Errorf("%s start_stream status after response: %w", stage, err)
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}
	return nil
}

func (d *Device) joinStartFailureWithRestoration(startErr error) error {
	restoreErr := d.restoreStreaming()
	if restoreErr != nil {
		restoreErr = fmt.Errorf("restoring streaming interface after start failure: %w", restoreErr)
	}
	d.stateMu.Lock()
	if d.lifecycle == deviceLifecycleStarting {
		if d.streamingPhase == StreamingInterfaceIdle {
			d.lifecycle = deviceLifecycleOpen
		} else {
			d.lifecycle = deviceLifecycleStopping
		}
	}
	d.stateMu.Unlock()
	return errors.Join(startErr, restoreErr)
}

func (d *Device) completeStreamingStart(ctx context.Context) error {
	d.stateMu.Lock()
	if d.lifecycle == deviceLifecycleStarting && context.Cause(ctx) == nil {
		d.lifecycle = deviceLifecycleOpen
		d.stateMu.Unlock()
		return nil
	}
	lifecycle := d.lifecycle
	cause := context.Cause(ctx)
	d.stateMu.Unlock()
	if cause == nil {
		cause = fmt.Errorf("device is %s", lifecycle)
	}
	return d.joinStartFailureWithRestoration(cause)
}

// StopStreaming stops the camera video stream.
func (d *Device) StopStreaming() error {
	d.lifecycleMu.Lock()
	defer d.lifecycleMu.Unlock()

	d.stateMu.Lock()
	switch d.lifecycle {
	case deviceLifecycleOpen:
		d.lifecycle = deviceLifecycleStopping
	case deviceLifecycleStopping:
	case deviceLifecycleClosing, deviceLifecycleClosed:
		lifecycle := d.lifecycle
		d.stateMu.Unlock()
		return fmt.Errorf("device is %s", lifecycle)
	default:
		lifecycle := d.lifecycle
		d.stateMu.Unlock()
		return fmt.Errorf("cannot stop device while %s", lifecycle)
	}
	cancels := d.lifecycleCancelSnapshotLocked()
	d.stateMu.Unlock()
	for _, cancel := range cancels {
		cancel(errStreamingStopped)
	}
	d.inFlight.Wait()

	d.controlMu.Lock()
	err := d.restoreStreaming()
	d.controlMu.Unlock()
	d.stateMu.Lock()
	if d.streamingPhase == StreamingInterfaceIdle {
		d.lifecycle = deviceLifecycleOpen
	}
	d.stateMu.Unlock()
	if err != nil {
		return err
	}
	return nil
}

func (d *Device) restoreStreaming() error {
	d.stateMu.Lock()
	phase := d.streamingPhase
	d.stateMu.Unlock()
	if phase == StreamingInterfaceIdle {
		return nil
	}

	streamingTransport, ok := d.transport.(USBStreamingTransport)
	if !ok {
		return fmt.Errorf("%w in phase %s", ErrStreamingUnsupported, phase)
	}

	var restoreErrors []error
	switch phase {
	case StreamingInterfaceRestorePending, StreamingInterfaceActive:
		err := streamingTransport.SetStreamingInterfaceIdle()
		switch {
		case err == nil:
		case errors.Is(err, ErrUSBNoDevice):
			restoreErrors = append(restoreErrors, fmt.Errorf("setting streaming interface idle: %w", err))
		default:
			return fmt.Errorf("setting streaming interface idle: %w", err)
		}
		d.stateMu.Lock()
		d.streamingPhase = StreamingInterfaceReleasePending
		d.stateMu.Unlock()
		fallthrough
	case StreamingInterfaceReleasePending:
		err := streamingTransport.ReleaseStreamingInterface()
		switch {
		case err == nil:
		case errors.Is(err, ErrUSBNoDevice), errors.Is(err, ErrUSBNotFound):
			restoreErrors = append(restoreErrors, fmt.Errorf("releasing streaming interface: %w", err))
		default:
			restoreErrors = append(restoreErrors, fmt.Errorf("releasing streaming interface: %w", err))
			return errors.Join(restoreErrors...)
		}
		d.stateMu.Lock()
		d.streamingPhase = StreamingInterfaceIdle
		d.stateMu.Unlock()
		return errors.Join(restoreErrors...)
	default:
		return fmt.Errorf("unknown streaming interface phase %d", phase)
	}
}

// IsStreaming returns whether the device is currently streaming.
func (d *Device) IsStreaming() bool {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	return d.streamingPhase == StreamingInterfaceActive
}

// sleepCtx sleeps for the given duration or returns early if ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-time.After(d):
		return nil
	}
}
