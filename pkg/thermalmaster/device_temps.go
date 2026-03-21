package thermalmaster

// DeviceTemps is kept for backward compatibility but is no longer used
// by GetDeviceTemp. The device returns a single unsigned uint16 temperature
// value, not two separate sensor/FPA values.
type DeviceTemps struct {
	Sensor float32
	FPA    float32
}
