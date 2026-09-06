//go:build !linux

package thermalmaster

import (
	"errors"
	"fmt"
	"strings"

	"github.com/xaionaro-go/usb"
)

func listGoUSBCameras() (_ []CameraInfo, _err error) {
	ctx, err := usb.NewContextWithOptions(usb.ContextOptions{})
	if err != nil {
		return nil, adaptGoUSBError("initializing USB", err)
	}
	defer func() {
		_err = errors.Join(_err, adaptGoUSBError("closing USB context", ctx.Close()))
	}()

	devices, err := ctx.OpenDevices(func(descriptor *usb.DeviceDesc) bool {
		if descriptor.Vendor != VendorID {
			return false
		}
		_, known := allConfigs[ProductID(descriptor.Product)]
		return known
	})
	defer func() {
		_err = errors.Join(_err, closeGoUSBDevices(devices, nil))
	}()
	if err != nil {
		return nil, adaptGoUSBError("enumerating USB devices", err)
	}

	cameras := make([]CameraInfo, 0, len(devices))
	for _, device := range devices {
		cfg := allConfigs[ProductID(device.Desc.Product)]
		cameras = append(cameras, CameraInfo{
			Model:   cfg.Model,
			Config:  cfg,
			Bus:     device.Desc.Bus,
			Address: device.Desc.Address,
		})
	}
	return cameras, nil
}

func openGoUSBDevice(oc openConfig) (_ *Device, _err error) {
	ctx, err := usb.NewContextWithOptions(usb.ContextOptions{})
	if err != nil {
		return nil, adaptGoUSBError("initializing USB", err)
	}
	contextTransferred := false
	defer func() {
		if !contextTransferred {
			_err = errors.Join(_err, adaptGoUSBError("closing USB context after setup failure", ctx.Close()))
		}
	}()

	devices, err := ctx.OpenDevices(func(descriptor *usb.DeviceDesc) bool {
		if descriptor.Vendor != VendorID {
			return false
		}
		if _, known := allConfigs[ProductID(descriptor.Product)]; !known {
			return false
		}
		if oc.filterBus && descriptor.Bus != oc.bus {
			return false
		}
		if oc.filterAddr && descriptor.Address != oc.address {
			return false
		}
		return true
	})
	var selected *usb.Device
	defer func() {
		_err = errors.Join(_err, closeGoUSBDevices(devices, selected))
	}()
	if err != nil && (len(devices) == 0 || !isUnavailableGoUSBDevice(err)) {
		return nil, adaptGoUSBError("finding USB device", err)
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("no ThermalMaster camera found")
	}

	selected, err = selectGoUSBDevice(devices, oc.serial)
	if err != nil {
		return nil, err
	}
	if selected == nil {
		return nil, fmt.Errorf("no camera matching serial %q", oc.serial)
	}
	if err := closeGoUSBDevices(devices, selected); err != nil {
		selected = nil
		return nil, err
	}
	operations := newGoUSBOperations(ctx, selected)
	defer func() {
		if _err != nil {
			_err = errors.Join(_err, operations.close())
		}
	}()

	transport, err := prepareGoUSBDevice(operations)
	if err != nil {
		return nil, err
	}
	contextTransferred = true
	cfg := allConfigs[ProductID(selected.Desc.Product)]
	dev := &Device{transport: transport, config: cfg}
	if info, err := dev.ReadDeviceInfo(); err == nil {
		dev.deviceType = DeviceTypeFromName(info.Model)
	}
	return dev, nil
}

func selectGoUSBDevice(
	devices []*usb.Device,
	serial string,
) (*usb.Device, error) {
	if serial == "" {
		return devices[0], nil
	}
	var errs []error
	for _, device := range devices {
		value, err := device.SerialNumber()
		if err != nil {
			// Another process or a disconnect can make one candidate
			// inaccessible while another camera still matches the request.
			if isUnavailableGoUSBDevice(err) {
				errs = append(errs, adaptGoUSBError("reading USB serial number", err))
				continue
			}
			return nil, adaptGoUSBError("reading USB serial number", err)
		}
		if strings.Contains(value, serial) {
			return device, nil
		}
	}
	return nil, errors.Join(errs...)
}

func isUnavailableGoUSBDevice(err error) bool {
	var native usb.Error
	if !errors.As(err, &native) {
		return false
	}
	switch native {
	case usb.ErrorAccess, usb.ErrorNoDevice, usb.ErrorBusy:
		return true
	default:
		return false
	}
}

func closeGoUSBDevices(
	devices []*usb.Device,
	keep *usb.Device,
) error {
	var errs []error
	for _, device := range devices {
		if device == keep {
			continue
		}
		if err := device.Close(); err != nil {
			errs = append(errs, adaptGoUSBError("closing enumerated USB device", err))
		}
	}
	return errors.Join(errs...)
}
