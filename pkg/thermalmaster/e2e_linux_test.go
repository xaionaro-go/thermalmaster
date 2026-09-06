//go:build linux && e2e_test

package thermalmaster

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestE2E_RequiredThreeReopenCyclesRestoreAltZero(t *testing.T) {
	if !e2eRequired(t) {
		t.Skip("set THERMALMASTER_E2E_REQUIRED=true for the strict reopen gate")
	}
	cameras, err := List()
	require.NoError(t, err)
	require.NotEmpty(t, cameras, "required ThermalMaster camera is unavailable")
	camera := cameras[0]
	selector := e2eSelector{Bus: camera.Bus, Address: camera.Address}
	skipped, err := runThreeReopenCycles(true, selector, productionE2ECycleEnvironment{})
	require.False(t, skipped)
	require.NoError(t, err)
}

type productionE2ECycleEnvironment struct{}

func (productionE2ECycleEnvironment) Open(selector e2eSelector) (e2eCycleDevice, error) {
	return Open(WithUSBAddress(selector.Bus, selector.Address))
}

func (productionE2ECycleEnvironment) RequireAltZero(
	selector e2eSelector,
	cycle int,
	stage string,
) error {
	return streamingAltZeroError(selector.Bus, selector.Address, cycle, stage)
}

func streamingAltZeroError(bus, address, cycle int, stage string) error {
	entries, err := os.ReadDir("/sys/bus/usb/devices")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		devicePath := filepath.Join("/sys/bus/usb/devices", entry.Name())
		busBytes, busErr := os.ReadFile(filepath.Join(devicePath, "busnum"))
		addressBytes, addressErr := os.ReadFile(filepath.Join(devicePath, "devnum"))
		if busErr != nil || addressErr != nil {
			continue
		}
		entryBus, busErr := strconv.Atoi(strings.TrimSpace(string(busBytes)))
		entryAddress, addressErr := strconv.Atoi(strings.TrimSpace(string(addressBytes)))
		if busErr != nil || addressErr != nil || entryBus != bus || entryAddress != address {
			continue
		}
		altPath := filepath.Join(
			"/sys/bus/usb/devices",
			entry.Name()+":1.1",
			"bAlternateSetting",
		)
		alt, err := os.ReadFile(altPath)
		if err != nil {
			return fmt.Errorf("cycle %d %s read %s: %w", cycle, stage, altPath, err)
		}
		if got := strings.TrimSpace(string(alt)); got != "0" {
			return fmt.Errorf("cycle %d %s alternate: got %q, want 0", cycle, stage, got)
		}
		return nil
	}
	return fmt.Errorf(
		"cycle %d %s: USB device bus=%d address=%d not found in sysfs",
		cycle,
		stage,
		bus,
		address,
	)
}
