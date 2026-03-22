package thermalmaster

import "fmt"

// CameraInfo describes a connected ThermalMaster camera found during enumeration.
type CameraInfo struct {
	Model   Model
	Config  ModelConfig
	Bus     int
	Address int
}

func (c CameraInfo) String() string {
	return fmt.Sprintf("model=%s bus=%d addr=%d", c.Config.Model, c.Bus, c.Address)
}
