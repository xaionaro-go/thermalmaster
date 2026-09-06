//go:build linux

package thermalmaster

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverLinuxUSBDevicesDoesNotReadUevent(t *testing.T) {
	sysfsPath := t.TempDir()
	devicePath := filepath.Join(t.TempDir(), "7-3")
	require.NoError(t, os.Mkdir(devicePath, 0o755))
	writeUSBAttribute(t, devicePath, "idVendor", "3474\n")
	writeUSBAttribute(t, devicePath, "idProduct", "45a2\n")
	writeUSBAttribute(t, devicePath, "busnum", "7\n")
	writeUSBAttribute(t, devicePath, "devnum", "19\n")
	writeUSBAttribute(t, devicePath, "serial", "P3025043DF123120418\n")
	require.NoError(t, os.WriteFile(filepath.Join(devicePath, "uevent"), nil, 0o000))
	require.NoError(t, os.Symlink(devicePath, filepath.Join(sysfsPath, "7-3")))

	unrelatedPath := filepath.Join(t.TempDir(), "8-2.4")
	require.NoError(t, os.Mkdir(unrelatedPath, 0o755))
	writeUSBAttribute(t, unrelatedPath, "idVendor", "05e3\n")
	writeUSBAttribute(t, unrelatedPath, "idProduct", "0625\n")
	require.NoError(t, os.Symlink(unrelatedPath, filepath.Join(sysfsPath, "8-2.4")))

	usbFS := &ueventRejectingFS{FS: os.DirFS(sysfsPath)}
	cameras, err := discoverLinuxUSBDevicesFS(usbFS)
	require.NoError(t, err)
	require.Len(t, cameras, 1)
	assert.False(t, usbFS.readAttempted)
	assert.Equal(t, ConfigP3, cameras[0].config)
	assert.Equal(t, 7, cameras[0].bus)
	assert.Equal(t, 19, cameras[0].address)
}

type ueventRejectingFS struct {
	fs.FS
	readAttempted bool
}

func (f *ueventRejectingFS) ReadFile(name string) ([]byte, error) {
	if strings.HasSuffix(name, "/uevent") || name == "uevent" {
		f.readAttempted = true
		return nil, assert.AnError
	}
	return fs.ReadFile(f.FS, name)
}

func writeUSBAttribute(t *testing.T, devicePath, name, value string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(devicePath, name), []byte(value), 0o644))
}
