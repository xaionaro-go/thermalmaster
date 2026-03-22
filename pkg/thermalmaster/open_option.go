package thermalmaster

// OpenOption configures device selection for Open.
type OpenOption interface {
	applyOpenOption(*openConfig)
}

type openConfig struct {
	serial     string
	bus        int
	address    int
	filterBus  bool
	filterAddr bool
}

// WithSerial filters for a device with the given serial number.
func WithSerial(serial string) OpenOption {
	return optionSerial(serial)
}

type optionSerial string

func (o optionSerial) applyOpenOption(c *openConfig) {
	c.serial = string(o)
}

// WithUSBAddress filters for a device at the given USB bus and address.
func WithUSBAddress(bus, address int) OpenOption {
	return optionUSBAddress{bus: bus, address: address}
}

type optionUSBAddress struct {
	bus     int
	address int
}

func (o optionUSBAddress) applyOpenOption(c *openConfig) {
	c.bus = o.bus
	c.address = o.address
	c.filterBus = true
	c.filterAddr = true
}

// WithUSBBus filters for a device on the given USB bus (any address).
func WithUSBBus(bus int) OpenOption {
	return optionUSBBus(bus)
}

type optionUSBBus int

func (o optionUSBBus) applyOpenOption(c *openConfig) {
	c.bus = int(o)
	c.filterBus = true
}
