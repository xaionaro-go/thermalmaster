package thermalmaster

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/gousb"
)

// selectDevice picks a device from the list based on the open config.
// If serial filtering is requested, each device is checked. Otherwise
// the first device is returned.
func selectDevice(devs []*gousb.Device, oc openConfig) *gousb.Device {
	if oc.serial == "" {
		return devs[0]
	}

	for _, d := range devs {
		d.SetAutoDetach(true)
		sn, err := d.SerialNumber()
		if err == nil && strings.Contains(sn, oc.serial) {
			return d
		}
	}
	return nil
}

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

// allConfigs lists all known model configurations keyed by PID.
var allConfigs = map[ProductID]ModelConfig{
	ConfigP3.PID: ConfigP3,
	ConfigP1.PID: ConfigP1,
}

// List enumerates all connected ThermalMaster cameras without opening them.
func List() ([]CameraInfo, error) {
	ctx := gousb.NewContext()
	defer ctx.Close()

	devs, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if desc.Vendor != gousb.ID(VendorID) {
			return false
		}
		_, known := allConfigs[ProductID(desc.Product)]
		return known
	})
	defer func() {
		for _, d := range devs {
			d.Close()
		}
	}()
	if err != nil {
		return nil, fmt.Errorf("enumerating USB devices: %w", err)
	}

	var result []CameraInfo
	for _, d := range devs {
		cfg := allConfigs[ProductID(d.Desc.Product)]
		result = append(result, CameraInfo{
			Model:   cfg.Model,
			Config:  cfg,
			Bus:     d.Desc.Bus,
			Address: d.Desc.Address,
		})
	}
	return result, nil
}

// Open opens a ThermalMaster camera via USB. Without options it opens the
// first camera found. Use WithSerial, WithUSBAddress, or WithUSBBus to
// select a specific device when multiple cameras are connected.
func Open(opts ...OpenOption) (_ *Device, _err error) {
	var oc openConfig
	for _, o := range opts {
		o.applyOpenOption(&oc)
	}

	usbCtx := gousb.NewContext()
	defer func() {
		if _err != nil {
			usbCtx.Close()
		}
	}()

	devs, err := usbCtx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if desc.Vendor != gousb.ID(VendorID) {
			return false
		}
		if _, known := allConfigs[ProductID(desc.Product)]; !known {
			return false
		}
		if oc.filterBus && desc.Bus != oc.bus {
			return false
		}
		if oc.filterAddr && desc.Address != oc.address {
			return false
		}
		return true
	})
	if err != nil && len(devs) == 0 {
		return nil, fmt.Errorf("finding device: %w", err)
	}

	if len(devs) == 0 {
		return nil, fmt.Errorf("no ThermalMaster camera found")
	}

	usbDev := selectDevice(devs, oc)

	// Close all devices we're not using.
	for _, d := range devs {
		if d != usbDev {
			d.Close()
		}
	}

	if usbDev == nil {
		return nil, fmt.Errorf("no camera matching serial %q", oc.serial)
	}
	defer func() {
		if _err != nil {
			usbDev.Close()
		}
	}()

	cfg := allConfigs[ProductID(usbDev.Desc.Product)]

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

	// Detect device type by reading the device name.
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
