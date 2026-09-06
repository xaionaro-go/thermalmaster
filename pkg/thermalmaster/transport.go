package thermalmaster

import (
	"context"
	"errors"
	"fmt"
)

// ErrStreamingUnsupported indicates that a USB transport supports camera
// control but cannot provide the streaming interface lifecycle.
var ErrStreamingUnsupported = errors.New("USB transport does not support streaming")

// ErrUSBNoDevice indicates that a USB operation cannot complete because the
// device is no longer connected.
var ErrUSBNoDevice = errors.New("USB device is no longer connected")

// ErrUSBNotFound indicates that a requested USB resource is not present. For
// interface release operations this means that the interface is not claimed.
var ErrUSBNotFound = errors.New("USB resource not found")

// LibUSBErrorCode is a native libusb error code.
type LibUSBErrorCode int

const (
	// LibUSBErrorNoDevice means that the device has been disconnected.
	LibUSBErrorNoDevice LibUSBErrorCode = -4
	// LibUSBErrorNotFound means that the requested entity was not found.
	LibUSBErrorNotFound LibUSBErrorCode = -5
	// LibUSBErrorTimeout means that an operation exceeded its transfer timeout.
	LibUSBErrorTimeout LibUSBErrorCode = -7
	// LibUSBErrorNotSupported means that an operation is unavailable on this platform.
	LibUSBErrorNotSupported LibUSBErrorCode = -12
)

// LibUSBError preserves the native operation and result code across wrapping
// and joined cleanup errors.
type LibUSBError struct {
	Operation string
	Code      LibUSBErrorCode
	Name      string
	// Cause retains the original usb error and any contextual wrapping.
	Cause error
}

// Error implements error.
func (e *LibUSBError) Error() string {
	if e.Name != "" {
		return fmt.Sprintf("%s: %s", e.Operation, e.Name)
	}
	return fmt.Sprintf("%s: libusb error %d", e.Operation, e.Code)
}

// Unwrap preserves the original cause and maps terminal result codes to portable errors.
func (e *LibUSBError) Unwrap() error {
	switch e.Code {
	case LibUSBErrorNoDevice:
		return errors.Join(e.Cause, ErrUSBNoDevice)
	case LibUSBErrorNotFound:
		return errors.Join(e.Cause, ErrUSBNotFound)
	default:
		return e.Cause
	}
}

// StreamingInterfacePhase describes the retryable lifecycle state of the USB
// interface used for frame streaming.
type StreamingInterfacePhase uint8

const (
	// StreamingInterfaceIdle means that the streaming interface is not claimed.
	StreamingInterfaceIdle StreamingInterfacePhase = iota
	// StreamingInterfaceActive means that the interface is claimed at its
	// streaming alternate setting.
	StreamingInterfaceActive
	// StreamingInterfaceReleasePending means that the interface is idle but
	// remains claimed and must still be released.
	StreamingInterfaceReleasePending
	// StreamingInterfaceRestorePending means that the streaming interface is
	// claimed, but selecting its active alternate setting did not complete.
	// A checked transition to the idle alternate setting is required before
	// release.
	StreamingInterfaceRestorePending
)

// String returns a human-readable name for the streaming interface phase.
func (p StreamingInterfacePhase) String() string {
	switch p {
	case StreamingInterfaceIdle:
		return "idle"
	case StreamingInterfaceActive:
		return "active"
	case StreamingInterfaceReleasePending:
		return "release pending"
	case StreamingInterfaceRestorePending:
		return "restore pending"
	default:
		return "unknown"
	}
}

// USBTransport abstracts USB communication for testability.
type USBTransport interface {
	// Control performs a USB control transfer.
	Control(
		requestType, request uint8,
		val, idx uint16,
		data []byte,
	) (int, error)
	// Close releases base USB resources. A streaming-capable transport must be
	// restored to StreamingInterfaceIdle before Close is called. Implementations
	// must leave each successfully completed or terminally attempted cleanup
	// step safe to retry: after an error, a later Close resumes the remaining
	// work without repeating already-consumed resource closes.
	Close() error
}

// USBStreamingTransport is an optional transport capability for reading frames
// and restoring the streaming interface through separately checked phases.
type USBStreamingTransport interface {
	USBTransport
	// BulkRead reads from the camera's streaming bulk endpoint.
	BulkRead(
		ctx context.Context,
		endpoint uint8,
		buf []byte,
	) (int, error)
	// ActivateStreamingInterface claims the streaming interface and selects its
	// active alternate setting. Claim success enters
	// StreamingInterfaceRestorePending; selecting the active alternate setting
	// then enters StreamingInterfaceActive. The returned phase is authoritative
	// even when the operation returns an error; a nil error must return
	// StreamingInterfaceActive.
	ActivateStreamingInterface() (StreamingInterfacePhase, error)
	// SetStreamingInterfaceIdle selects the idle alternate setting without
	// releasing the interface. When called in StreamingInterfaceRestorePending
	// or StreamingInterfaceActive, success enters
	// StreamingInterfaceReleasePending. Device disappearance also advances to
	// release-pending while returning its terminal error; other failures preserve
	// the current phase for retry.
	SetStreamingInterfaceIdle() error
	// ReleaseStreamingInterface releases an already-idle streaming interface.
	// It must not be called from StreamingInterfaceRestorePending or
	// StreamingInterfaceActive. When called in StreamingInterfaceReleasePending,
	// success enters StreamingInterfaceIdle. Device disappearance and a missing
	// interface claim also enter idle while returning their terminal error; other
	// failures remain release-pending for retry.
	ReleaseStreamingInterface() error
}
