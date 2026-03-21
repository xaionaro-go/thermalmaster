package thermalmaster

// USB constants for ThermalMaster cameras.
const (
	VendorID         = 0x3474
	MarkerSize       = 12
	bulkEndpointAddr = 0x81 // Bulk IN endpoint address for frame data
	bulkEndpointNum  = 1    // Bulk IN endpoint number within the interface
	usbConfigNum     = 1    // USB configuration number
	controlIntf      = 0    // USB interface for control transfers
	controlAlt       = 0    // Alt setting for control interface
)

// Temperature conversion constants.
const (
	TempScale    = 64    // Raw values are in 1/64 Kelvin
	KelvinOffset = 273.15
)
