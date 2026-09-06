//go:build linux

package thermalmaster

import (
	"errors"
	"fmt"
	"os"

	"github.com/xaionaro-go/usb"
)

type linuxUSBTransport struct {
	*goUSBTransport
	deviceFile *os.File
}

func openLinuxUSBTransport(devicePath string) (_ *linuxUSBTransport, _err error) {
	deviceFile, err := os.OpenFile(devicePath, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	var (
		ctx        *usb.Context
		operations *goUSBOperations
	)
	defer func() {
		if _err == nil {
			return
		}
		var cleanupErr error
		switch {
		case operations != nil:
			cleanupErr = operations.close()
		case ctx != nil:
			cleanupErr = adaptGoUSBError("closing USB context after setup failure", ctx.Close())
		}
		_err = errors.Join(_err, cleanupErr, wrapError("closing usbfs device after setup failure", deviceFile.Close()))
	}()

	ctx, err = usb.NewContextWithOptions(usb.ContextOptions{DeviceDiscovery: usb.DisableDeviceDiscovery})
	if err != nil {
		return nil, adaptGoUSBError("initializing USB without discovery", err)
	}
	device, err := ctx.OpenDeviceWithFileDescriptor(deviceFile.Fd())
	if err != nil {
		return nil, adaptGoUSBError("wrapping usbfs device", err)
	}
	operations = newGoUSBOperations(ctx, device)
	transport, err := prepareGoUSBDevice(operations)
	if err != nil {
		return nil, err
	}
	return &linuxUSBTransport{
		goUSBTransport: transport,
		deviceFile:     deviceFile,
	}, nil
}

func (t *linuxUSBTransport) Close() error {
	baseErr := t.goUSBTransport.Close()
	if !t.goUSBTransport.closed {
		return baseErr
	}

	var fileErr error
	if t.deviceFile != nil {
		deviceFile := t.deviceFile
		t.deviceFile = nil
		if err := deviceFile.Close(); err != nil {
			fileErr = fmt.Errorf("closing usbfs device: %w", err)
		}
	}
	return errors.Join(baseErr, fileErr)
}

var _ USBStreamingTransport = (*linuxUSBTransport)(nil)
