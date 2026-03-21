package thermalmaster

import "fmt"

// SetEmissivity sets the emissivity value for environment correction.
func (d *Device) SetEmissivity(value EmissivityValue) error {
	return d.SendCommandNoResponse(commandWithRegister(CmdSetEnvEMS, uint16(value)))
}

// GetEmissivity reads the current emissivity value.
func (d *Device) GetEmissivity() (EmissivityValue, error) {
	resp, err := d.SendCommandWithResponse(CmdGetEnvEMS, 1)
	if err != nil {
		return 0, fmt.Errorf("getting emissivity: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing emissivity response: %w", err)
	}

	return EmissivityValue(v), nil
}

// SetEnvTA sets the ambient temperature parameter (TA) for environment correction.
func (d *Device) SetEnvTA(value EnvTemperatureValue) error {
	return d.SendCommandNoResponse(commandWithRegister(CmdSetEnvTA, uint16(value)))
}

// GetEnvTA reads the current ambient temperature parameter (TA).
func (d *Device) GetEnvTA() (EnvTemperatureValue, error) {
	resp, err := d.SendCommandWithResponse(CmdGetEnvTA, 1)
	if err != nil {
		return 0, fmt.Errorf("getting env TA: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing env TA response: %w", err)
	}

	return EnvTemperatureValue(v), nil
}

// SetEnvTU sets the reflected temperature parameter (TU) for environment correction.
func (d *Device) SetEnvTU(value EnvTemperatureValue) error {
	return d.SendCommandNoResponse(commandWithRegister(CmdSetEnvTU, uint16(value)))
}

// GetEnvTU reads the current reflected temperature parameter (TU).
func (d *Device) GetEnvTU() (EnvTemperatureValue, error) {
	resp, err := d.SendCommandWithResponse(CmdGetEnvTU, 1)
	if err != nil {
		return 0, fmt.Errorf("getting env TU: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing env TU response: %w", err)
	}

	return EnvTemperatureValue(v), nil
}

// SetEnvTAU sets the atmospheric transmittance parameter (TAU) for environment
// correction.
func (d *Device) SetEnvTAU(value EnvTransmittance) error {
	return d.SendCommandNoResponse(commandWithRegister(CmdSetEnvTAU, uint16(value)))
}

// GetEnvTAU reads the current atmospheric transmittance parameter (TAU).
func (d *Device) GetEnvTAU() (EnvTransmittance, error) {
	resp, err := d.SendCommandWithResponse(CmdGetEnvTAU, 1)
	if err != nil {
		return 0, fmt.Errorf("getting env TAU: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("parsing env TAU response: %w", err)
	}

	return EnvTransmittance(v), nil
}

// SetEnvCorrectionEnabled enables or disables environment correction.
func (d *Device) SetEnvCorrectionEnabled(enabled bool) error {
	v := uint16(0)
	if enabled {
		v = 1
	}
	return d.SendCommandNoResponse(commandWithRegister(CmdSetEnvCorrectSwitch, v))
}

// GetEnvCorrectionEnabled reads whether environment correction is enabled.
func (d *Device) GetEnvCorrectionEnabled() (bool, error) {
	resp, err := d.SendCommandWithResponse(CmdGetEnvCorrectSwitch, 1)
	if err != nil {
		return false, fmt.Errorf("getting env correction switch: %w", err)
	}

	v, err := parseSingleByteResponse(resp)
	if err != nil {
		return false, fmt.Errorf("parsing env correction switch response: %w", err)
	}

	return v != 0, nil
}
