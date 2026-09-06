//go:build !linux

package thermalmaster

func listCameras() ([]CameraInfo, error) {
	return listGoUSBCameras()
}

func openDevice(oc openConfig) (*Device, error) {
	return openGoUSBDevice(oc)
}
