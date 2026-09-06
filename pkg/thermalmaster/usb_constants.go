package thermalmaster

import "time"

// USB constants for ThermalMaster cameras.
const (
	VendorID                     = 0x3474
	MarkerSize                   = 12
	bulkEndpointAddr             = 0x81 // Bulk IN endpoint address for frame data
	usbConfigNum                 = 1    // USB configuration number
	controlIntf                  = 0    // USB interface for control transfers
	controlAlt                   = 0    // Alt setting for control interface
	usbControlTransferTimeout    = time.Second
	usbStartupPrimingReadTimeout = 100 * time.Millisecond
)

// Temperature conversion constants.
const (
	TempScale    = 64 // Raw values are in 1/64 Kelvin
	KelvinOffset = 273.15
)
