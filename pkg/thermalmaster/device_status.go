package thermalmaster

// DeviceStatus represents the device current status register value.
type DeviceStatus uint16

const (
	DeviceStatusStartup  DeviceStatus = 0
	DeviceStatusPreview  DeviceStatus = 1
	DeviceStatusVideoOut DeviceStatus = 2
	DeviceStatusUpdate   DeviceStatus = 3
	DeviceStatusError    DeviceStatus = 4
)
