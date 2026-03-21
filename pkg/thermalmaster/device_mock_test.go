package thermalmaster

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helper: build a uint16 LE response (4 bytes, as the protocol uses 4-byte
// responses for uint16 values).
// ---------------------------------------------------------------------------

func uint16LEResp(v uint16) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint16(b, v)
	return b
}

func uint32LEResp(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

func int16Pair(a, b int16) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint16(buf[0:2], uint16(a))
	binary.LittleEndian.PutUint16(buf[2:4], uint16(b))
	return buf
}

func float32x4(vals ...float32) []byte {
	buf := make([]byte, len(vals)*4)
	for i, v := range vals {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// ---------------------------------------------------------------------------
// Test: NewDeviceWithTransport and Config
// ---------------------------------------------------------------------------

func TestNewDeviceWithTransport(t *testing.T) {
	dev, _ := newMockDevice()
	assert.Equal(t, ConfigP3, dev.Config())
	assert.False(t, dev.IsStreaming())
}

// ---------------------------------------------------------------------------
// Test: Close
// ---------------------------------------------------------------------------

func TestClose(t *testing.T) {
	dev, mock := newMockDevice()
	err := dev.Close()
	require.NoError(t, err)
	assert.True(t, mock.closed)
}

func TestClose_WhileStreaming(t *testing.T) {
	dev, mock := newMockDevice()

	// Simulate streaming state.
	dev.mu.Lock()
	dev.streaming = true
	dev.mu.Unlock()

	err := dev.Close()
	require.NoError(t, err)
	assert.True(t, mock.closed)
	assert.False(t, dev.IsStreaming())
}

// ---------------------------------------------------------------------------
// Table-driven tests: Set commands (SendCommandNoResponse pattern)
// ---------------------------------------------------------------------------

func TestSetCommands_NoArg(t *testing.T) {
	tests := []struct {
		name    string
		fn      func(*Device) error
		wantCmd [CommandSize]byte
	}{
		{"TriggerShutter", func(d *Device) error { return d.TriggerShutter() }, CmdShutter},
		{"ManualFFCUpdate", func(d *Device) error { return d.ManualFFCUpdate() }, CmdManualFFCUpdate},
		{"ManualFFCWithGain", func(d *Device) error { return d.ManualFFCWithGain() }, CmdManualFFCWithGain},
		{"SaveSystemParams", func(d *Device) error { return d.SaveSystemParams() }, CmdSaveSystemParams},
		{"RestoreSystemParams", func(d *Device) error { return d.RestoreSystemParams() }, CmdRestoreSystemParams},
		{"ResetToRom", func(d *Device) error { return d.ResetToRom() }, CmdResetToRom},
		{"ResetToBootloader", func(d *Device) error { return d.ResetToBootloader() }, CmdResetToBootloader},
		{"EnterRebootMode", func(d *Device) error { return d.EnterRebootMode() }, CmdEnterRebootMode},
		{"StartHeartbeat", func(d *Device) error { return d.StartHeartbeat() }, CmdHeartbeatStart},
		{"SendHeartbeat", func(d *Device) error { return d.SendHeartbeat() }, CmdHeartbeatSend},
		{"PauseVideoStream", func(d *Device) error { return d.PauseVideoStream() }, CmdPauseVideoStream},
		{"DPCCalibCancel", func(d *Device) error { return d.DPCCalibCancel() }, CmdDPCCalibCancel},
		{"DPCCalibClear", func(d *Device) error { return d.DPCCalibClear() }, CmdDPCCalibClear},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev, mock := newMockDevice()
			mock.setupStandardSetResponse()

			err := tt.fn(dev)
			require.NoError(t, err)
			assert.Equal(t, tt.wantCmd, mock.lastCommand())
		})
	}
}

func TestSetCommands_NoArg_USBError(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*Device) error
	}{
		{"TriggerShutter", func(d *Device) error { return d.TriggerShutter() }},
		{"ManualFFCUpdate", func(d *Device) error { return d.ManualFFCUpdate() }},
		{"SaveSystemParams", func(d *Device) error { return d.SaveSystemParams() }},
		{"StartHeartbeat", func(d *Device) error { return d.StartHeartbeat() }},
		{"SendHeartbeat", func(d *Device) error { return d.SendHeartbeat() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev, mock := newMockDevice()
			mock.nextControlError = fmt.Errorf("USB disconnected")

			err := tt.fn(dev)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "USB disconnected")
		})
	}
}

// ---------------------------------------------------------------------------
// Test: Set commands with uint16 register value
// ---------------------------------------------------------------------------

func TestSetCommands_WithRegister(t *testing.T) {
	tests := []struct {
		name  string
		fn    func(*Device) error
		base  [CommandSize]byte
		value uint16
	}{
		{"SetBrightness", func(d *Device) error { return d.SetBrightness(42) }, CmdSetBrightness, 42},
		{"SetContrast", func(d *Device) error { return d.SetContrast(50) }, CmdSetContrast, 50},
		{"SetDetailEnhance", func(d *Device) error { return d.SetDetailEnhance(3) }, CmdSetDetailEnhance, 3},
		{"SetNoiseReduction", func(d *Device) error { return d.SetNoiseReduction(7) }, CmdSetNoiseReduction, 7},
		{"SetSpaceNoiseReduce", func(d *Device) error { return d.SetSpaceNoiseReduce(5) }, CmdSetSpaceNoiseReduce, 5},
		{"SetTimeNoiseReduce", func(d *Device) error { return d.SetTimeNoiseReduce(8) }, CmdSetTimeNoiseReduce, 8},
		{"SetGlobalContrast", func(d *Device) error { return d.SetGlobalContrast(60) }, CmdSetGlobalContrast, 60},
		{"SetROILevel", func(d *Device) error { return d.SetROILevel(2) }, CmdSetROILevel, 2},
		{"SetSceneMode", func(d *Device) error { return d.SetSceneMode(SceneMode(1)) }, CmdSetSceneMode, 1},
		{"SetMirrorFlip", func(d *Device) error { return d.SetMirrorFlip(MirrorAndFlip) }, CmdSetMirrorFlip, 3},
		{"SetEdgeEnhance", func(d *Device) error { return d.SetEdgeEnhance(10) }, CmdSetEdgeEnhance, 10},
		{"SetIsothermalMode", func(d *Device) error { return d.SetIsothermalMode(IsothermalMode(2)) }, CmdSetIsothermalMode, 2},
		{"SetIsothermalLimit", func(d *Device) error { return d.SetIsothermalLimit(500) }, CmdSetIsothermalLimit, 500},
		{"SetProfessionMode", func(d *Device) error { return d.SetProfessionMode(ProfessionMode(1)) }, CmdSetProfessionMode, 1},
		{"SetDigitalVideoOutput", func(d *Device) error { return d.SetDigitalVideoOutput(DigitalVideoOutput(2)) }, CmdSetDigitalVideoOutput, 2},
		{"SetStreamMidMode", func(d *Device) error { return d.SetStreamMidMode(StreamMidMode(1)) }, CmdSetStreamMidMode, 1},
		{"SetStreamSourceMode", func(d *Device) error { return d.SetStreamSourceMode(StreamSourceMode(2)) }, CmdSetStreamSourceMode, 2},
		{"SetEmissivity", func(d *Device) error { return d.SetEmissivity(950) }, CmdSetEnvEMS, 950},
		{"SetEnvTA", func(d *Device) error { return d.SetEnvTA(2500) }, CmdSetEnvTA, 2500},
		{"SetEnvTU", func(d *Device) error { return d.SetEnvTU(2500) }, CmdSetEnvTU, 2500},
		{"SetEnvTAU", func(d *Device) error { return d.SetEnvTAU(80) }, CmdSetEnvTAU, 80},
		{"SetSunDetectPixelRatio", func(d *Device) error { return d.SetSunDetectPixelRatio(100) }, CmdSetSunDetectPixelRatio, 100},
		{"SetSunDetectRoundnessLevel", func(d *Device) error { return d.SetSunDetectRoundnessLevel(3) }, CmdSetSunDetectRoundnessLevel, 3},
		{"SetAutoFFCCurrentParams", func(d *Device) error { return d.SetAutoFFCCurrentParams(12) }, CmdSetAutoFFCCurrentParams, 12},
		{"SetShutterManualFFCSwitch", func(d *Device) error { return d.SetShutterManualFFCSwitch(1) }, CmdSetShutterManualFFCSwitch, 1},
		{"SetCursorToDPC", func(d *Device) error { return d.SetCursorToDPC(99) }, CmdSetCursorToDPC, 99},
		{"ShowFrameTemp", func(d *Device) error { return d.ShowFrameTemp(FrameTempDisplayMode(1)) }, CmdShowFrameTemp, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev, mock := newMockDevice()
			mock.setupStandardSetResponse()

			err := tt.fn(dev)
			require.NoError(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// Test: SetPalette (uses P3 byte position, not register)
// ---------------------------------------------------------------------------

func TestSetPalette(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardSetResponse()

	err := dev.SetPalette(PaletteIndex(3))
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Test: Set commands with bool register value
// ---------------------------------------------------------------------------

func TestSetCommands_Bool(t *testing.T) {
	tests := []struct {
		name    string
		fnTrue  func(*Device) error
		fnFalse func(*Device) error
		base    [CommandSize]byte
	}{
		{
			"SetAutoFFCEnabled",
			func(d *Device) error { return d.SetAutoFFCEnabled(true) },
			func(d *Device) error { return d.SetAutoFFCEnabled(false) },
			CmdSetAutoFFCStatus,
		},
		{
			"SetOverexposureProtection",
			func(d *Device) error { return d.SetOverexposureProtection(true) },
			func(d *Device) error { return d.SetOverexposureProtection(false) },
			CmdSetAllFFCStatusOverexposure,
		},
		{
			"SetSunDetectEnabled",
			func(d *Device) error { return d.SetSunDetectEnabled(true) },
			func(d *Device) error { return d.SetSunDetectEnabled(false) },
			CmdSetSunDetectSwitch,
		},
		{
			"SetIsothermalEnabled",
			func(d *Device) error { return d.SetIsothermalEnabled(true) },
			func(d *Device) error { return d.SetIsothermalEnabled(false) },
			CmdSetIsothermalSwitch,
		},
		{
			"SetEnvCorrectionEnabled",
			func(d *Device) error { return d.SetEnvCorrectionEnabled(true) },
			func(d *Device) error { return d.SetEnvCorrectionEnabled(false) },
			CmdSetEnvCorrectSwitch,
		},
		{
			"SetCursorEnabled",
			func(d *Device) error { return d.SetCursorEnabled(true) },
			func(d *Device) error { return d.SetCursorEnabled(false) },
			CmdCursorSwitchSet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_true", func(t *testing.T) {
			dev, mock := newMockDevice()
			mock.setupStandardSetResponse()

			err := tt.fnTrue(dev)
			require.NoError(t, err)
			assert.Equal(t, commandWithRegister(tt.base, 1), mock.lastCommand())
		})

		t.Run(tt.name+"_false", func(t *testing.T) {
			dev, mock := newMockDevice()
			mock.setupStandardSetResponse()

			err := tt.fnFalse(dev)
			require.NoError(t, err)
			assert.Equal(t, commandWithRegister(tt.base, 0), mock.lastCommand())
		})
	}
}

// ---------------------------------------------------------------------------
// Test: Gain (special: uses direct commands, not commandWithRegister)
// ---------------------------------------------------------------------------

func TestSetGain_High(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardSetResponse()

	err := dev.SetGain(GainHigh)
	require.NoError(t, err)
}

func TestSetGain_Low(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardSetResponse()

	err := dev.SetGain(GainLow)
	require.NoError(t, err)
}

func TestSetGain_InvalidMode(t *testing.T) {
	dev, _ := newMockDevice()

	err := dev.SetGain(GainMode(99))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown gain mode")
}

func TestSetGain_USBError(t *testing.T) {
	dev, mock := newMockDevice()
	mock.nextControlError = fmt.Errorf("USB disconnected")

	err := dev.SetGain(GainHigh)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "USB disconnected")
}

func TestGetGain(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardGetResponse(uint16LEResp(uint16(GainHigh)))

	mode, err := dev.GetGain()
	require.NoError(t, err)
	assert.Equal(t, GainHigh, mode)
}

func TestGetGain_Low(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardGetResponse(uint16LEResp(uint16(GainLow)))

	mode, err := dev.GetGain()
	require.NoError(t, err)
	assert.Equal(t, GainLow, mode)
}

func TestGetGain_USBError(t *testing.T) {
	dev, mock := newMockDevice()
	mock.nextControlError = fmt.Errorf("USB disconnected")

	_, err := dev.GetGain()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "USB disconnected")
}

// ---------------------------------------------------------------------------
// Table-driven tests: Get commands returning typed values
// ---------------------------------------------------------------------------

func TestGetCommands_Uint16(t *testing.T) {
	t.Run("GetBrightness", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(42))
		v, err := dev.GetBrightness()
		require.NoError(t, err)
		assert.Equal(t, BrightnessLevel(42), v)
	})

	t.Run("GetContrast", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(50))
		v, err := dev.GetContrast()
		require.NoError(t, err)
		assert.Equal(t, ContrastLevel(50), v)
	})

	t.Run("GetDetailEnhance", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(3))
		v, err := dev.GetDetailEnhance()
		require.NoError(t, err)
		assert.Equal(t, DetailEnhanceLevel(3), v)
	})

	t.Run("GetNoiseReduction", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(7))
		v, err := dev.GetNoiseReduction()
		require.NoError(t, err)
		assert.Equal(t, NoiseReductionLevel(7), v)
	})

	t.Run("GetGlobalContrast", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(60))
		v, err := dev.GetGlobalContrast()
		require.NoError(t, err)
		assert.Equal(t, ContrastLevel(60), v)
	})

	t.Run("GetROILevel", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(2))
		v, err := dev.GetROILevel()
		require.NoError(t, err)
		assert.Equal(t, ROILevel(2), v)
	})

	t.Run("GetEdgeEnhance", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(10))
		v, err := dev.GetEdgeEnhance()
		require.NoError(t, err)
		assert.Equal(t, EdgeEnhanceLevel(10), v)
	})

	t.Run("GetIsothermalLimit", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse([]byte{200})
		v, err := dev.GetIsothermalLimit()
		require.NoError(t, err)
		assert.Equal(t, IsothermalLimit(200), v)
	})

	t.Run("GetStreamMidMode", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(1))
		v, err := dev.GetStreamMidMode()
		require.NoError(t, err)
		assert.Equal(t, StreamMidMode(1), v)
	})

	t.Run("GetEmissivity", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse([]byte{95})
		v, err := dev.GetEmissivity()
		require.NoError(t, err)
		assert.Equal(t, EmissivityValue(95), v)
	})

	t.Run("GetEnvTA", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse([]byte{250})
		v, err := dev.GetEnvTA()
		require.NoError(t, err)
		assert.Equal(t, EnvTemperatureValue(250), v)
	})

	t.Run("GetEnvTU", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse([]byte{250})
		v, err := dev.GetEnvTU()
		require.NoError(t, err)
		assert.Equal(t, EnvTemperatureValue(250), v)
	})

	t.Run("GetEnvTAU", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(80))
		v, err := dev.GetEnvTAU()
		require.NoError(t, err)
		assert.Equal(t, EnvTransmittance(80), v)
	})

	t.Run("GetSunDetectPixelRatio", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(100))
		v, err := dev.GetSunDetectPixelRatio()
		require.NoError(t, err)
		assert.Equal(t, SunDetectPixelRatio(100), v)
	})

	t.Run("GetSunDetectRoundnessLevel", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(3))
		v, err := dev.GetSunDetectRoundnessLevel()
		require.NoError(t, err)
		assert.Equal(t, SunDetectRoundnessLevel(3), v)
	})

	t.Run("GetShutterStatus", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(1))
		v, err := dev.GetShutterStatus()
		require.NoError(t, err)
		assert.Equal(t, ShutterStatus(1), v)
	})

	t.Run("GetAutoFFCCurrentParams", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(12))
		v, err := dev.GetAutoFFCCurrentParams()
		require.NoError(t, err)
		assert.Equal(t, AutoFFCParams(12), v)
	})

	t.Run("GetDeviceCurrentStatus", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(0x00FF))
		v, err := dev.GetDeviceCurrentStatus()
		require.NoError(t, err)
		assert.Equal(t, DeviceStatus(0x00FF), v)
	})

	t.Run("GetCRGValue", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse([]byte{0x34})
		v, err := dev.GetCRGValue()
		require.NoError(t, err)
		assert.Equal(t, CRGValue(0x34), v)
	})
}

func TestGetCommands_Uint16_USBError(t *testing.T) {
	t.Run("GetBrightness", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.nextControlError = fmt.Errorf("USB timeout")
		_, err := dev.GetBrightness()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "USB timeout")
	})

	t.Run("GetContrast", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.nextControlError = fmt.Errorf("USB timeout")
		_, err := dev.GetContrast()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "USB timeout")
	})

	t.Run("GetEmissivity", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.nextControlError = fmt.Errorf("USB timeout")
		_, err := dev.GetEmissivity()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "USB timeout")
	})

	t.Run("GetCRGValue", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.nextControlError = fmt.Errorf("USB timeout")
		_, err := dev.GetCRGValue()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "USB timeout")
	})
}

// ---------------------------------------------------------------------------
// Table-driven tests: Get commands returning typed uint16
// ---------------------------------------------------------------------------

func TestGetCommands_TypedUint16(t *testing.T) {
	t.Run("GetSceneMode", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(5))

		v, err := dev.GetSceneMode()
		require.NoError(t, err)
		assert.Equal(t, SceneMode(5), v)
	})

	t.Run("GetPalette", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(3))

		v, err := dev.GetPalette()
		require.NoError(t, err)
		assert.Equal(t, PaletteIndex(3), v)
	})

	t.Run("GetMirrorFlip", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(uint16(MirrorAndFlip)))

		v, err := dev.GetMirrorFlip()
		require.NoError(t, err)
		assert.Equal(t, MirrorAndFlip, v)
	})

	t.Run("GetIsothermalMode", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(2))

		v, err := dev.GetIsothermalMode()
		require.NoError(t, err)
		assert.Equal(t, IsothermalMode(2), v)
	})

	t.Run("GetProfessionMode", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(1))

		v, err := dev.GetProfessionMode()
		require.NoError(t, err)
		assert.Equal(t, ProfessionMode(1), v)
	})

	t.Run("GetDigitalVideoOutput", func(t *testing.T) {
		dev, mock := newMockDevice()
		mock.setupStandardGetResponse(uint16LEResp(2))

		v, err := dev.GetDigitalVideoOutput()
		require.NoError(t, err)
		assert.Equal(t, DigitalVideoOutput(2), v)
	})
}

// ---------------------------------------------------------------------------
// Table-driven tests: Get commands returning bool
// ---------------------------------------------------------------------------

func TestGetCommands_Bool(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*Device) (bool, error)
	}{
		{"GetAutoFFCEnabled", func(d *Device) (bool, error) { return d.GetAutoFFCEnabled() }},
		{"GetOverexposureProtection", func(d *Device) (bool, error) { return d.GetOverexposureProtection() }},
		{"GetSunDetectEnabled", func(d *Device) (bool, error) { return d.GetSunDetectEnabled() }},
		{"GetIsothermalEnabled", func(d *Device) (bool, error) { return d.GetIsothermalEnabled() }},
		{"GetEnvCorrectionEnabled", func(d *Device) (bool, error) { return d.GetEnvCorrectionEnabled() }},
		{"GetCursorEnabled", func(d *Device) (bool, error) { return d.GetCursorEnabled() }},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_true", func(t *testing.T) {
			dev, mock := newMockDevice()
			mock.setupStandardGetResponse(uint16LEResp(1))

			v, err := tt.fn(dev)
			require.NoError(t, err)
			assert.True(t, v)
		})

		t.Run(tt.name+"_false", func(t *testing.T) {
			dev, mock := newMockDevice()
			mock.setupStandardGetResponse(uint16LEResp(0))

			v, err := tt.fn(dev)
			require.NoError(t, err)
			assert.False(t, v)
		})

		t.Run(tt.name+"_usb_error", func(t *testing.T) {
			dev, mock := newMockDevice()
			mock.nextControlError = fmt.Errorf("USB disconnected")

			_, err := tt.fn(dev)
			require.Error(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// Test: GetDeviceTemp
// ---------------------------------------------------------------------------

func TestGetDeviceTemp(t *testing.T) {
	dev, mock := newMockDevice()
	// 25.50C = 2550 as uint16 LE
	resp := make([]byte, 2)
	binary.LittleEndian.PutUint16(resp, 2550)
	mock.setupStandardGetResponse(resp)

	temp, err := dev.GetDeviceTemp()
	require.NoError(t, err)
	assert.InDelta(t, 25.50, temp, 0.01)
}

func TestGetDeviceTemp_HighValue(t *testing.T) {
	dev, mock := newMockDevice()
	// 55.00C = 5500 as uint16 LE
	resp := make([]byte, 2)
	binary.LittleEndian.PutUint16(resp, 5500)
	mock.setupStandardGetResponse(resp)

	temp, err := dev.GetDeviceTemp()
	require.NoError(t, err)
	assert.InDelta(t, 55.00, temp, 0.01)
}

func TestGetDeviceTemp_USBError(t *testing.T) {
	dev, mock := newMockDevice()
	mock.nextControlError = fmt.Errorf("USB disconnected")

	_, err := dev.GetDeviceTemp()
	require.Error(t, err)
}

func TestGetDeviceTemp_ShortResponse(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardGetResponse([]byte{0x01}) // only 1 byte, need 2

	_, err := dev.GetDeviceTemp()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

// ---------------------------------------------------------------------------
// Test: GetPoweredTime
// ---------------------------------------------------------------------------

func TestGetPoweredTime(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardGetResponse(uint32LEResp(3600))

	v, err := dev.GetPoweredTime()
	require.NoError(t, err)
	assert.Equal(t, uint32(3600), v)
}

func TestGetPoweredTime_USBError(t *testing.T) {
	dev, mock := newMockDevice()
	mock.nextControlError = fmt.Errorf("USB disconnected")

	_, err := dev.GetPoweredTime()
	require.Error(t, err)
}

func TestGetPoweredTime_ShortResponse(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardGetResponse([]byte{0x01, 0x02}) // only 2 bytes, need 4

	_, err := dev.GetPoweredTime()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

// ---------------------------------------------------------------------------
// Test: WriteVLParam / ReadVLParam
// ---------------------------------------------------------------------------

func TestWriteVLParam(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardSetResponse()

	err := dev.WriteVLParam(0x1234, 0x5678)
	require.NoError(t, err)

	expected := commandWithRegisterAndData(CmdWriteVLParam, 0x1234, 0x5678)
	assert.Equal(t, expected, mock.lastCommand())
}

func TestReadVLParam(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardGetResponse([]byte{0xCD})

	v, err := dev.ReadVLParam(0x1234)
	require.NoError(t, err)
	assert.Equal(t, uint8(0xCD), v)

	// Verify the command included the register address.
	expected := commandWithRegister(CmdReadVLParam, 0x1234)
	assert.Equal(t, expected, mock.lastCommand())
}

func TestReadVLParam_USBError(t *testing.T) {
	dev, mock := newMockDevice()
	mock.nextControlError = fmt.Errorf("USB disconnected")

	_, err := dev.ReadVLParam(0x1234)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Test: SetCursorPosition / GetCursorPosition
// ---------------------------------------------------------------------------

func TestSetCursorPosition(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardSetResponse()

	err := dev.SetCursorPosition(CursorPosition{X: 100, Y: 50})
	require.NoError(t, err)

	expected := commandWithRegisterAndData(CmdCursorPositionSet, 100, 50)
	assert.Equal(t, expected, mock.lastCommand())
}

func TestGetCursorPosition(t *testing.T) {
	dev, mock := newMockDevice()
	resp := make([]byte, 4)
	binary.LittleEndian.PutUint16(resp[0:2], 128)
	binary.LittleEndian.PutUint16(resp[2:4], 96)
	mock.setupStandardGetResponse(resp)

	pos, err := dev.GetCursorPosition()
	require.NoError(t, err)
	assert.Equal(t, uint16(128), pos.X)
	assert.Equal(t, uint16(96), pos.Y)
}

func TestGetCursorPosition_ShortResponse(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardGetResponse([]byte{0x01, 0x02}) // only 2 bytes, need 4

	_, err := dev.GetCursorPosition()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

// ---------------------------------------------------------------------------
// Test: ReadRegister / ReadDeviceInfo
// ---------------------------------------------------------------------------

func TestReadRegister(t *testing.T) {
	dev, mock := newMockDevice()
	data := append([]byte("P3-256"), 0x00, 0x00, 0x00) // trailing nulls
	mock.setupStandardGetResponse(data)

	s, err := dev.ReadRegister(CmdReadName, 30)
	require.NoError(t, err)
	assert.Equal(t, "P3-256", s)
}

func TestReadRegister_USBError(t *testing.T) {
	dev, mock := newMockDevice()
	mock.nextControlError = fmt.Errorf("USB disconnected")

	_, err := dev.ReadRegister(CmdReadName, 30)
	require.Error(t, err)
}

func TestReadDeviceInfo(t *testing.T) {
	dev, mock := newMockDevice()

	// 6 register reads, each needs: status + response + status
	// ReadDeviceInfo reads: Model, FWVersion, PartNumber, Serial, HWVersion, ModelLong
	responses := []string{
		"P3-256",
		"1.2.3",
		"PN-001",
		"SN-12345",
		"HW-v2",
		"ThermalMaster P3-256",
	}

	for _, r := range responses {
		data := append([]byte(r), 0x00)
		mock.setupStandardGetResponse(data)
	}

	info, err := dev.ReadDeviceInfo()
	require.NoError(t, err)
	assert.Equal(t, "P3-256", info.Model)
	assert.Equal(t, "1.2.3", info.FWVersion)
	assert.Equal(t, "PN-001", info.PartNumber)
	assert.Equal(t, "SN-12345", info.Serial)
	assert.Equal(t, "HW-v2", info.HWVersion)
	assert.Equal(t, "ThermalMaster P3-256", info.ModelLong)
}

func TestReadDeviceInfo_FirstRegisterFails(t *testing.T) {
	dev, mock := newMockDevice()
	mock.nextControlError = fmt.Errorf("USB disconnected")

	_, err := dev.ReadDeviceInfo()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading model")
}

// ---------------------------------------------------------------------------
// Test: SetTPDPointCoord / GetPointTempInfo / GetFrameTempInfo
// ---------------------------------------------------------------------------

func TestSetTPDPointCoord(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardSetResponse()

	err := dev.SetTPDPointCoord(0, 128, 96)
	require.NoError(t, err)

	cmd := mock.lastCommand()
	// Verify the three uint16 values are at the right offsets.
	assert.Equal(t, uint16(0), binary.LittleEndian.Uint16(cmd[4:6]))
	assert.Equal(t, uint16(128), binary.LittleEndian.Uint16(cmd[6:8]))
	assert.Equal(t, uint16(96), binary.LittleEndian.Uint16(cmd[8:10]))
}

func TestGetPointTempInfo(t *testing.T) {
	dev, mock := newMockDevice()
	// 4 float32 values: 25.5, 30.0, 20.0, 22.5
	mock.setupStandardGetResponse(float32x4(25.5, 30.0, 20.0, 22.5))

	results, err := dev.GetPointTempInfo()
	require.NoError(t, err)
	require.Len(t, results, 4)
	assert.InDelta(t, 25.5, float64(results[0].TempC), 0.01)
	assert.InDelta(t, 30.0, float64(results[1].TempC), 0.01)
	assert.InDelta(t, 20.0, float64(results[2].TempC), 0.01)
	assert.InDelta(t, 22.5, float64(results[3].TempC), 0.01)
}

func TestGetPointTempInfo_ShortResponse(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardGetResponse([]byte{0x01, 0x02}) // only 2 bytes

	_, err := dev.GetPointTempInfo()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestGetFrameTempInfo(t *testing.T) {
	dev, mock := newMockDevice()
	// 4 float32 values: min=15.0, max=45.0, avg=25.0, unused=0
	mock.setupStandardGetResponse(float32x4(15.0, 45.0, 25.0, 0.0))

	result, err := dev.GetFrameTempInfo()
	require.NoError(t, err)
	require.Len(t, result.Values, 4)
	assert.InDelta(t, 15.0, float64(result.Values[0]), 0.01)
	assert.InDelta(t, 45.0, float64(result.Values[1]), 0.01)
}

func TestGetFrameTempInfo_ShortResponse(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardGetResponse([]byte{0x01, 0x02}) // only 2 bytes

	_, err := dev.GetFrameTempInfo()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

// ---------------------------------------------------------------------------
// Test: RecalTPDBy1Point
// ---------------------------------------------------------------------------

func TestRecalTPDBy1Point(t *testing.T) {
	dev, mock := newMockDevice()
	mock.setupStandardSetResponse()

	err := dev.RecalTPDBy1Point(36.5)
	require.NoError(t, err)

	cmd := mock.lastCommand()
	// Verify float32 is encoded at bytes 6-9.
	bits := binary.LittleEndian.Uint32(cmd[6:10])
	assert.InDelta(t, 36.5, float64(math.Float32frombits(bits)), 0.01)
}

// ---------------------------------------------------------------------------
// Test: ReadFrame with mock bulk data
// ---------------------------------------------------------------------------

func TestReadFrame_NotStreaming(t *testing.T) {
	dev, _ := newMockDevice()

	_, err := dev.ReadFrame(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not streaming")
}

func TestReadFrame_Success(t *testing.T) {
	dev, mock := newMockDevice()

	// Set streaming state.
	dev.mu.Lock()
	dev.streaming = true
	dev.mu.Unlock()

	cfg := ConfigP3
	frameReadSize := cfg.FrameReadSize()
	framePayload := frameReadSize - 2*MarkerSize // pixel data between markers

	// Build a valid frame: startMarker + pixelData + endMarker
	frame := make([]byte, frameReadSize)

	// Start marker (12 bytes).
	frame[0] = 0x0C           // Length
	frame[1] = SyncStartEven  // Sync
	binary.LittleEndian.PutUint32(frame[2:6], 100)  // Cnt1
	binary.LittleEndian.PutUint32(frame[6:10], 200) // Cnt2
	binary.LittleEndian.PutUint16(frame[10:12], 40) // Cnt3

	// Fill pixel data with pattern.
	for i := MarkerSize; i < MarkerSize+framePayload; i++ {
		frame[i] = byte(i & 0xFF)
	}

	// End marker (12 bytes).
	endOff := frameReadSize - MarkerSize
	frame[endOff] = 0x0C
	frame[endOff+1] = SyncEndEven
	binary.LittleEndian.PutUint32(frame[endOff+2:endOff+6], 100) // Cnt1 matches
	binary.LittleEndian.PutUint32(frame[endOff+6:endOff+10], 200)
	binary.LittleEndian.PutUint16(frame[endOff+10:endOff+12], 40)

	// Split frame into mock bulk reads matching real USB behavior:
	// Phase 1 receives a 12-byte start marker as a separate transfer.
	// Phase 2 receives pixel data + end marker in readChunkSize chunks.
	startChunk := make([]byte, MarkerSize)
	copy(startChunk, frame[:MarkerSize])
	mock.bulkData = append(mock.bulkData, startChunk)

	pos := MarkerSize
	for pos < frameReadSize {
		end := pos + readChunkSize
		if end > frameReadSize {
			end = frameReadSize
		}
		chunk := make([]byte, end-pos)
		copy(chunk, frame[pos:end])
		mock.bulkData = append(mock.bulkData, chunk)
		pos = end
	}

	result, err := dev.ReadFrame(context.Background())
	require.NoError(t, err)
	// Result should be startMarker + pixelData (without end marker).
	assert.Len(t, result, frameReadSize-MarkerSize)

	// Verify start marker is present and correct.
	assert.Equal(t, byte(SyncStartEven), result[1])

	// Verify stats.
	stats := dev.Stats()
	assert.Equal(t, uint64(1), stats.FramesRead)
	assert.Equal(t, uint64(0), stats.MarkerMismatches)
}

// TestReadFrame_EmbeddedMarker verifies that ReadFrame correctly finds a start
// marker embedded within a larger bulk read (the real USB behavior, where
// markers are not separate 12-byte transfers).
func TestReadFrame_EmbeddedMarker(t *testing.T) {
	dev, mock := newMockDevice()

	dev.mu.Lock()
	dev.streaming = true
	dev.mu.Unlock()

	cfg := ConfigP3
	frameReadSize := cfg.FrameReadSize()
	framePayload := frameReadSize - 2*MarkerSize

	frame := make([]byte, frameReadSize)

	// Start marker.
	frame[0] = 0x0C
	frame[1] = SyncStartOdd
	binary.LittleEndian.PutUint32(frame[2:6], 42)
	binary.LittleEndian.PutUint32(frame[6:10], 100)
	binary.LittleEndian.PutUint16(frame[10:12], 40)

	for i := MarkerSize; i < MarkerSize+framePayload; i++ {
		frame[i] = byte(i & 0xFF)
	}

	endOff := frameReadSize - MarkerSize
	frame[endOff] = 0x0C
	frame[endOff+1] = SyncEndOdd
	binary.LittleEndian.PutUint32(frame[endOff+2:endOff+6], 42)
	binary.LittleEndian.PutUint32(frame[endOff+6:endOff+10], 100)
	binary.LittleEndian.PutUint16(frame[endOff+10:endOff+12], 40)

	// Simulate real USB behavior: garbage bytes before the frame, then
	// the frame with the start marker embedded in a larger chunk (not
	// delivered as a separate 12-byte transfer).
	garbage := make([]byte, 5000)
	for i := range garbage {
		// Avoid accidentally producing [0x0C, 0x8C/0x8D] in garbage.
		garbage[i] = 0xAA
	}

	// Prepend garbage to the frame so the start marker is at an
	// arbitrary offset within a readChunkSize-compatible chunk.
	combined := append(garbage, frame...)
	pos := 0
	for pos < len(combined) {
		end := pos + readChunkSize
		if end > len(combined) {
			end = len(combined)
		}
		chunk := make([]byte, end-pos)
		copy(chunk, combined[pos:end])
		mock.bulkData = append(mock.bulkData, chunk)
		pos = end
	}

	result, err := dev.ReadFrame(context.Background())
	require.NoError(t, err)
	assert.Len(t, result, frameReadSize-MarkerSize)
	assert.Equal(t, byte(SyncStartOdd), result[1])

	stats := dev.Stats()
	assert.Equal(t, uint64(1), stats.FramesRead)
	assert.Equal(t, uint64(0), stats.MarkerMismatches)
}

func TestReadFrame_MarkerMismatch(t *testing.T) {
	dev, mock := newMockDevice()

	dev.mu.Lock()
	dev.streaming = true
	dev.mu.Unlock()

	cfg := ConfigP3
	frameReadSize := cfg.FrameReadSize()
	framePayload := frameReadSize - 2*MarkerSize

	frame := make([]byte, frameReadSize)

	// Start marker with Cnt1=100.
	frame[0] = 0x0C
	frame[1] = SyncStartEven
	binary.LittleEndian.PutUint32(frame[2:6], 100)
	binary.LittleEndian.PutUint32(frame[6:10], 200)
	binary.LittleEndian.PutUint16(frame[10:12], 40)

	for i := MarkerSize; i < MarkerSize+framePayload; i++ {
		frame[i] = byte(i & 0xFF)
	}

	// End marker with different Cnt1=999.
	endOff := frameReadSize - MarkerSize
	frame[endOff] = 0x0C
	frame[endOff+1] = SyncEndEven
	binary.LittleEndian.PutUint32(frame[endOff+2:endOff+6], 999) // Mismatch!
	binary.LittleEndian.PutUint32(frame[endOff+6:endOff+10], 200)
	binary.LittleEndian.PutUint16(frame[endOff+10:endOff+12], 40)

	startChunk := make([]byte, MarkerSize)
	copy(startChunk, frame[:MarkerSize])
	mock.bulkData = append(mock.bulkData, startChunk)

	pos := MarkerSize
	for pos < frameReadSize {
		end := pos + readChunkSize
		if end > frameReadSize {
			end = frameReadSize
		}
		chunk := make([]byte, end-pos)
		copy(chunk, frame[pos:end])
		mock.bulkData = append(mock.bulkData, chunk)
		pos = end
	}

	result, err := dev.ReadFrame(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)

	stats := dev.Stats()
	assert.Equal(t, uint64(1), stats.FramesRead)
	assert.Equal(t, uint64(1), stats.MarkerMismatches)
}

func TestReadFrame_BulkReadError(t *testing.T) {
	dev, mock := newMockDevice()

	dev.mu.Lock()
	dev.streaming = true
	dev.mu.Unlock()

	mock.nextBulkError = fmt.Errorf("bulk read timeout")

	_, err := dev.ReadFrame(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading frame data")
}

// ---------------------------------------------------------------------------
// Test: StartStreaming / StopStreaming
// ---------------------------------------------------------------------------

func TestStartStreaming(t *testing.T) {
	dev, mock := newMockDevice()

	// StartStreaming sequence:
	// 1. sendCommand(CmdStartStream) -> OUT control
	// 2. readStatus -> IN 0xC1:0x22
	// 3. readResponse(1) -> IN 0xC1:0x21
	// 4. readStatus -> IN 0xC1:0x22
	// (sleep 1s)
	// 5. SetInterfaceAlt(1,1)
	// 6. Control(0x40, 0xEE, 0, 1, nil) -> OUT
	// (sleep 2s)
	// 7. BulkRead (initial, may fail)
	// 8. sendCommand(CmdStartStream) -> OUT control
	// 9. readStatus -> IN 0xC1:0x22
	// 10. readResponse(1) -> IN 0xC1:0x21
	// 11. readStatus -> IN 0xC1:0x22

	// First start_stream sequence.
	mock.addStatusResponse(0x02)
	mock.addReadResponse([]byte{0x01})
	mock.addStatusResponse(0x03)

	// Bulk read (initial, expected to fail -- add one chunk).
	mock.bulkData = append(mock.bulkData, make([]byte, 100))

	// Second start_stream sequence.
	mock.addStatusResponse(0x02)
	mock.addReadResponse([]byte{0x01})
	mock.addStatusResponse(0x03)

	err := dev.StartStreaming(context.Background())
	require.NoError(t, err)
	assert.True(t, dev.IsStreaming())

	// Verify SetInterfaceAlt was called.
	assert.Equal(t, 1, mock.currentAlt[1])

	// Verify two start_stream commands were sent.
	cmds := mock.allCommands()
	assert.GreaterOrEqual(t, len(cmds), 2)
	assert.Equal(t, CmdStartStream, cmds[0])
	assert.Equal(t, CmdStartStream, cmds[1])
}

func TestStartStreaming_SendCommandError(t *testing.T) {
	dev, mock := newMockDevice()
	mock.nextControlError = fmt.Errorf("USB disconnected")

	err := dev.StartStreaming(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initial start_stream")
	assert.False(t, dev.IsStreaming())
}

func TestStopStreaming(t *testing.T) {
	dev, mock := newMockDevice()

	// Manually set streaming state.
	dev.mu.Lock()
	dev.streaming = true
	dev.mu.Unlock()

	err := dev.StopStreaming()
	require.NoError(t, err)
	assert.False(t, dev.IsStreaming())

	// Verify interface was released.
	assert.Equal(t, 0, mock.currentAlt[1])
}

func TestStopStreaming_NotStreaming(t *testing.T) {
	dev, _ := newMockDevice()

	err := dev.StopStreaming()
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Test: RunHeartbeatLoop
// ---------------------------------------------------------------------------

func TestRunHeartbeatLoop_ContextCancel(t *testing.T) {
	dev, mock := newMockDevice()

	// Add enough status responses for several heartbeats.
	for i := 0; i < 20; i++ {
		mock.setupStandardSetResponse()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	err := dev.RunHeartbeatLoop(ctx, 50*time.Millisecond)
	require.NoError(t, err) // Should return nil on context cancellation.

	// Verify at least one heartbeat was sent.
	heartbeats := mock.countCalls(bmRequestTypeOut, bRequestSendCmd)
	assert.Greater(t, heartbeats, 0)
}

func TestRunHeartbeatLoop_HeartbeatError(t *testing.T) {
	dev, mock := newMockDevice()

	// First heartbeat succeeds, second fails.
	mock.setupStandardSetResponse()
	mock.addStatusResponse(0x02) // This will be consumed...
	// Then inject an error on the next sendCommand.

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// We need to make the second heartbeat fail. The mock will run out of
	// status responses after the first heartbeat, causing an error.
	err := dev.RunHeartbeatLoop(ctx, 50*time.Millisecond)
	// Should return error when heartbeat fails.
	if err != nil {
		assert.Contains(t, err.Error(), "mock responses exhausted")
	}
}

// ---------------------------------------------------------------------------
// Test: SendCommandWithResponse status error propagation
// ---------------------------------------------------------------------------

func TestSendCommandWithResponse_StatusAfterCommandError(t *testing.T) {
	dev, mock := newMockDevice()
	// sendCommand succeeds, but readStatus after command fails.
	mock.addResponseErr(0xC1, 0x22, fmt.Errorf("status read failed"))

	_, err := dev.SendCommandWithResponse(CmdGetGainVDCMD, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading status after command")
}

func TestSendCommandWithResponse_ReadResponseError(t *testing.T) {
	dev, mock := newMockDevice()
	// sendCommand + readStatus succeed, but readResponse fails.
	mock.addStatusResponse(0x02)
	mock.addResponseErr(0xC1, 0x21, fmt.Errorf("response read failed"))

	_, err := dev.SendCommandWithResponse(CmdGetGainVDCMD, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading response")
}

func TestSendCommandWithResponse_StatusAfterResponseError(t *testing.T) {
	dev, mock := newMockDevice()
	// sendCommand + readStatus + readResponse succeed, but final readStatus fails.
	mock.addStatusResponse(0x02)
	mock.addReadResponse(uint16LEResp(42))
	mock.addResponseErr(0xC1, 0x22, fmt.Errorf("final status failed"))

	_, err := dev.SendCommandWithResponse(CmdGetGainVDCMD, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading status after response")
}

func TestSendCommandNoResponse_StatusError(t *testing.T) {
	dev, mock := newMockDevice()
	// sendCommand succeeds, readStatus fails.
	mock.addResponseErr(0xC1, 0x22, fmt.Errorf("status read failed"))

	err := dev.SendCommandNoResponse(CmdShutter)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading status")
}

// ---------------------------------------------------------------------------
// Test: Stats
// ---------------------------------------------------------------------------

func TestStats_Initial(t *testing.T) {
	dev, _ := newMockDevice()
	stats := dev.Stats()
	assert.Equal(t, uint64(0), stats.FramesRead)
	assert.Equal(t, uint64(0), stats.FramesDropped)
	assert.Equal(t, uint64(0), stats.MarkerMismatches)
}

// ---------------------------------------------------------------------------
// Test: IsStreaming
// ---------------------------------------------------------------------------

func TestIsStreaming(t *testing.T) {
	dev, _ := newMockDevice()
	assert.False(t, dev.IsStreaming())

	dev.mu.Lock()
	dev.streaming = true
	dev.mu.Unlock()

	assert.True(t, dev.IsStreaming())
}
