package thermalmaster

import "fmt"

// TempUnit selects the temperature display unit for the legend.
type TempUnit int

const (
	TempCelsius    TempUnit = iota
	TempFahrenheit
	TempRaw
)

// FormatValue formats a RawThermalValue according to the unit.
func (u TempUnit) FormatValue(raw RawThermalValue) string {
	switch u {
	case TempCelsius:
		return fmt.Sprintf("%.1f\u00b0C", raw.Celsius())
	case TempFahrenheit:
		c := raw.Celsius()
		return fmt.Sprintf("%.1f\u00b0F", c*9.0/5.0+32.0)
	default:
		return fmt.Sprintf("%d", raw)
	}
}
