package thermalmaster

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/gousb"
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
	transport  USBTransport
	config     ModelConfig
	deviceType DeviceType
	streaming  bool
	stats      FrameStats
	mu         sync.Mutex
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

// Open opens a ThermalMaster camera of the given model via USB.
func Open(model Model) (_ *Device, _err error) {
	var cfg ModelConfig
	switch model {
	case ModelP3:
		cfg = ConfigP3
	case ModelP1:
		cfg = ConfigP1
	default:
		return nil, fmt.Errorf("unknown model: %d", model)
	}

	usbCtx := gousb.NewContext()
	defer func() {
		if _err != nil {
			usbCtx.Close()
		}
	}()

	usbDev, err := usbCtx.OpenDeviceWithVIDPID(gousb.ID(VendorID), gousb.ID(uint16(cfg.PID)))
	if err != nil {
		return nil, fmt.Errorf("finding device: %w", err)
	}
	if usbDev == nil {
		return nil, fmt.Errorf("device not found (VID=%04x PID=%04x)", VendorID, cfg.PID)
	}
	defer func() {
		if _err != nil {
			usbDev.Close()
		}
	}()

	if err := usbDev.SetAutoDetach(true); err != nil {
		return nil, fmt.Errorf("setting auto-detach: %w", err)
	}

	usbCfg, err := usbDev.Config(usbConfigNum)
	if err != nil {
		return nil, fmt.Errorf("claiming USB config %d: %w", usbConfigNum, err)
	}
	defer func() {
		if _err != nil {
			usbCfg.Close()
		}
	}()

	intf0, err := usbCfg.Interface(controlIntf, controlAlt)
	if err != nil {
		return nil, fmt.Errorf("claiming interface %d: %w", controlIntf, err)
	}

	transport := &goUSBTransport{
		ctx:   usbCtx,
		dev:   usbDev,
		cfg:   usbCfg,
		intf0: intf0,
	}

	dev := &Device{
		transport: transport,
		config:    cfg,
	}

	// Detect device type by reading the device name, matching the logic in
	// the vendor's get_current_device_type() logic.
	info, err := dev.ReadDeviceInfo()
	if err == nil {
		dev.deviceType = DeviceTypeFromName(info.Model)
	}

	return dev, nil
}

// Close releases all USB resources held by the device.
func (d *Device) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.streaming {
		d.stopStreamingLocked()
	}

	return d.transport.Close()
}

// Config returns the model configuration for this device.
func (d *Device) Config() ModelConfig {
	return d.config
}

// Stats returns a snapshot of frame statistics.
func (d *Device) Stats() FrameStats {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.stats
}

// sendCommand sends an 18-byte command via USB control transfer.
func (d *Device) sendCommand(cmd [CommandSize]byte) error {
	_, err := d.transport.Control(bmRequestTypeOut, bRequestSendCmd, 0, 0, cmd[:])
	return err
}

// readResponse reads a response of the given length via USB control transfer.
func (d *Device) readResponse(length int) ([]byte, error) {
	buf := make([]byte, length)
	n, err := d.transport.Control(bmRequestTypeIn, bRequestReadResp, 0, 0, buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// readStatus polls the status register until the camera is ready.
// Matches the usb_status_check_done loop in the vendor's USB transport:
// status=1 means busy (retry after 1ms), status>=2 means done.
func (d *Device) readStatus() (byte, error) {
	buf := make([]byte, 1)
	for range statusPollLimit {
		_, err := d.transport.Control(bmRequestTypeIn, bRequestReadStatus, 0, 0, buf)
		if err != nil {
			return 0, err
		}

		if buf[0] != 1 {
			return buf[0], nil
		}

		// Status 1 = busy. Wait 1ms before polling again, matching the
		// vendor's usleep(1000) in usb_status_check_done.
		time.Sleep(time.Millisecond)
	}
	return buf[0], fmt.Errorf("status poll timeout (stuck at busy)")
}

// SendCommandWithResponse sends a command, reads status, reads response, reads
// status again. This is the standard read pattern for the P3 protocol.
func (d *Device) SendCommandWithResponse(
	cmd [CommandSize]byte,
	respLen int,
) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.sendCommand(cmd); err != nil {
		return nil, fmt.Errorf("sending command: %w", err)
	}
	if _, err := d.readStatus(); err != nil {
		return nil, fmt.Errorf("reading status after command: %w", err)
	}

	resp, err := d.readResponse(respLen)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if _, err := d.readStatus(); err != nil {
		return nil, fmt.Errorf("reading status after response: %w", err)
	}
	return resp, nil
}

// SendCommandNoResponse sends a command and reads status only (no response data).
func (d *Device) SendCommandNoResponse(cmd [CommandSize]byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.sendCommand(cmd); err != nil {
		return fmt.Errorf("sending command: %w", err)
	}
	if _, err := d.readStatus(); err != nil {
		return fmt.Errorf("reading status: %w", err)
	}
	return nil
}
