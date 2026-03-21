package thermalmaster

import "fmt"

// ErrUnsupportedCommand is returned when a command is not supported by the
// current device type.
type ErrUnsupportedCommand struct {
	Command    string
	DeviceType DeviceType
}

func (e ErrUnsupportedCommand) Error() string {
	return fmt.Sprintf("command %q not supported on device type %d", e.Command, e.DeviceType)
}
