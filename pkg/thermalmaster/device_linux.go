//go:build linux

package thermalmaster

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	linuxUSBSysfsPath = "/sys/bus/usb/devices"
	linuxUSBDevfsPath = "/dev/bus/usb"
)

type linuxUSBDevice struct {
	config  ModelConfig
	serial  string
	bus     int
	address int
}

func listCameras() ([]CameraInfo, error) {
	return listLinuxCameras(linuxUSBSysfsPath)
}

func listLinuxCameras(sysfsPath string) ([]CameraInfo, error) {
	devices, err := discoverLinuxUSBDevices(sysfsPath)
	if err != nil {
		return nil, err
	}

	cameras := make([]CameraInfo, 0, len(devices))
	for _, device := range devices {
		cameras = append(cameras, CameraInfo{
			Model:   device.config.Model,
			Config:  device.config,
			Bus:     device.bus,
			Address: device.address,
		})
	}
	return cameras, nil
}

func openDevice(oc openConfig) (*Device, error) {
	devices, err := discoverLinuxUSBDevices(linuxUSBSysfsPath)
	if err != nil {
		return nil, err
	}

	selected := selectLinuxUSBDevice(devices, oc)
	if selected == nil {
		if oc.serial != "" {
			return nil, fmt.Errorf("no camera matching serial %q", oc.serial)
		}
		return nil, fmt.Errorf("no ThermalMaster camera found")
	}

	devicePath := filepath.Join(
		linuxUSBDevfsPath,
		fmt.Sprintf("%03d", selected.bus),
		fmt.Sprintf("%03d", selected.address),
	)
	transport, err := openLinuxUSBTransport(devicePath)
	if err != nil {
		return nil, fmt.Errorf("opening camera at USB bus %d address %d: %w", selected.bus, selected.address, err)
	}

	dev := &Device{
		transport: transport,
		config:    selected.config,
	}
	if info, err := dev.ReadDeviceInfo(); err == nil {
		dev.deviceType = DeviceTypeFromName(info.Model)
	}
	return dev, nil
}

func discoverLinuxUSBDevices(sysfsPath string) ([]linuxUSBDevice, error) {
	return discoverLinuxUSBDevicesFS(os.DirFS(sysfsPath))
}

func discoverLinuxUSBDevicesFS(sysfs fs.FS) ([]linuxUSBDevice, error) {
	entries, err := fs.ReadDir(sysfs, ".")
	if err != nil {
		return nil, fmt.Errorf("reading USB sysfs directory: %w", err)
	}

	var devices []linuxUSBDevice
	for _, entry := range entries {
		vendor, err := readLinuxUSBInteger(sysfs, entry.Name(), "idVendor", 16)
		if err != nil || vendor != VendorID {
			continue
		}

		product, err := readLinuxUSBInteger(sysfs, entry.Name(), "idProduct", 16)
		if err != nil {
			continue
		}
		cfg, known := allConfigs[ProductID(product)]
		if !known {
			continue
		}

		bus, err := readLinuxUSBInteger(sysfs, entry.Name(), "busnum", 10)
		if err != nil {
			continue
		}
		address, err := readLinuxUSBInteger(sysfs, entry.Name(), "devnum", 10)
		if err != nil {
			continue
		}
		serial, _ := fs.ReadFile(sysfs, filepath.Join(entry.Name(), "serial"))

		devices = append(devices, linuxUSBDevice{
			config:  cfg,
			serial:  strings.TrimSpace(string(serial)),
			bus:     bus,
			address: address,
		})
	}
	return devices, nil
}

func readLinuxUSBInteger(sysfs fs.FS, devicePath, name string, base int) (int, error) {
	value, err := fs.ReadFile(sysfs, filepath.Join(devicePath, name))
	if err != nil {
		return 0, err
	}
	parsed, err := strconv.ParseUint(strings.TrimSpace(string(value)), base, 16)
	if err != nil {
		return 0, err
	}
	return int(parsed), nil
}

func selectLinuxUSBDevice(devices []linuxUSBDevice, oc openConfig) *linuxUSBDevice {
	for idx := range devices {
		device := &devices[idx]
		if oc.filterBus && device.bus != oc.bus {
			continue
		}
		if oc.filterAddr && device.address != oc.address {
			continue
		}
		if oc.serial != "" && !strings.Contains(device.serial, oc.serial) {
			continue
		}
		return device
	}
	return nil
}
