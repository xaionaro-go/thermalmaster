package thermalmaster

import (
	"errors"
	"fmt"

	"github.com/google/gousb"
)

// goUSBTransport wraps gousb and implements USBTransport.
type goUSBTransport struct {
	ctx   *gousb.Context
	dev   *gousb.Device
	cfg   *gousb.Config
	intf0 *gousb.Interface
	intf1 *gousb.Interface
	ep    *gousb.InEndpoint
}

func (t *goUSBTransport) Control(
	requestType, request uint8,
	val, idx uint16,
	data []byte,
) (int, error) {
	return t.dev.Control(requestType, request, val, idx, data)
}

func (t *goUSBTransport) BulkRead(endpoint uint8, buf []byte) (int, error) {
	if t.ep == nil {
		return 0, fmt.Errorf("streaming endpoint not configured")
	}
	return t.ep.Read(buf)
}

func (t *goUSBTransport) SetInterfaceAlt(intf, alt int) error {
	if intf != streamingIntf {
		return fmt.Errorf("unsupported interface: %d", intf)
	}

	if alt == streamingAltIdle {
		// Release streaming interface.
		if t.intf1 != nil {
			t.intf1.Close()
			t.intf1 = nil
			t.ep = nil
		}
		return nil
	}

	// Release any previously claimed streaming interface to avoid leaking it.
	if t.intf1 != nil {
		t.intf1.Close()
		t.intf1 = nil
		t.ep = nil
	}

	// Claim interface with alt setting.
	intf1, err := t.cfg.Interface(intf, alt)
	if err != nil {
		return fmt.Errorf("claiming interface %d alt %d: %w", intf, alt, err)
	}
	t.intf1 = intf1

	// Set up bulk IN endpoint.
	ep, err := intf1.InEndpoint(bulkEndpointNum)
	if err != nil {
		return fmt.Errorf("getting IN endpoint 1: %w", err)
	}
	t.ep = ep
	return nil
}

func (t *goUSBTransport) Close() error {
	var errs []error
	if t.intf1 != nil {
		t.intf1.Close()
		t.intf1 = nil
		t.ep = nil
	}
	if t.intf0 != nil {
		t.intf0.Close()
		t.intf0 = nil
	}
	if t.cfg != nil {
		if err := t.cfg.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing USB config: %w", err))
		}
		t.cfg = nil
	}
	if t.dev != nil {
		if err := t.dev.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing USB device: %w", err))
		}
		t.dev = nil
	}
	if t.ctx != nil {
		if err := t.ctx.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing USB context: %w", err))
		}
		t.ctx = nil
	}

	return errors.Join(errs...)
}
