package thermalmaster

import (
	"context"
	"fmt"
	"time"
)

// Streaming protocol constants.
const (
	streamingIntf     = 1                // USB interface number for bulk streaming
	streamingAltIdle  = 0                // Alt setting: no streaming
	streamingAltStart = 1                // Alt setting: streaming active
	preInterfaceDelay = 1 * time.Second  // Delay before interface configuration
	preStreamDelay    = 2 * time.Second  // Delay for camera readiness after 0xEE
)

// StartStreaming starts the camera video stream following the initialization
// sequence from P3_PROTOCOL.md.
func (d *Device) StartStreaming(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.streaming {
		return fmt.Errorf("already streaming")
	}

	d.stats = FrameStats{}

	// 1. Send start_stream command with status checks.
	if err := d.sendCommand(CmdStartStream); err != nil {
		return fmt.Errorf("initial start_stream: %w", err)
	}
	d.readStatus()    // reads 0x02
	d.readResponse(1) // reads 0x01 or 0x35 (restart)
	d.readStatus()    // reads 0x03

	// 2. Wait before configuring interface.
	if err := sleepCtx(ctx, preInterfaceDelay); err != nil {
		return err
	}

	// 3. Claim interface 1 with alt setting 1 for streaming.
	if err := d.transport.SetInterfaceAlt(streamingIntf, streamingAltStart); err != nil {
		return fmt.Errorf("claiming streaming interface: %w", err)
	}

	// 4. Send 0xEE control transfer to start the stream.
	_, err := d.transport.Control(bmRequestTypeDevOut, bRequestStartStream, 0, streamingIntf, nil)
	if err != nil {
		return fmt.Errorf("sending start stream (0xEE): %w", err)
	}

	// 5. Wait for camera to be ready.
	if err := sleepCtx(ctx, preStreamDelay); err != nil {
		return err
	}

	// 6. Issue initial bulk read (may timeout, that's expected).
	buf := make([]byte, d.config.FrameSize())
	d.transport.BulkRead(bulkEndpointAddr, buf) // ignore error, expected to timeout

	// 7. Final start_stream.
	if err := d.sendCommand(CmdStartStream); err != nil {
		return fmt.Errorf("final start_stream: %w", err)
	}
	d.readStatus()
	d.readResponse(1)
	d.readStatus()

	d.streaming = true
	return nil
}

// StopStreaming stops the camera video stream.
func (d *Device) StopStreaming() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.stopStreamingLocked()
}

func (d *Device) stopStreamingLocked() error {
	if !d.streaming {
		return nil
	}
	d.transport.SetInterfaceAlt(streamingIntf, streamingAltIdle)
	d.streaming = false
	return nil
}

// IsStreaming returns whether the device is currently streaming.
func (d *Device) IsStreaming() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.streaming
}

// sleepCtx sleeps for the given duration or returns early if ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
