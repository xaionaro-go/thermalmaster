package thermalmaster

// USBTransport abstracts USB communication for testability.
type USBTransport interface {
	// Control performs a USB control transfer.
	Control(
		requestType, request uint8,
		val, idx uint16,
		data []byte,
	) (int, error)
	// BulkRead reads from a bulk endpoint.
	BulkRead(endpoint uint8, buf []byte) (int, error)
	// SetInterfaceAlt sets the alternate setting for a USB interface.
	SetInterfaceAlt(intf, alt int) error
	// Close releases all USB resources.
	Close() error
}
