//go:build !linux

package nvme

import "fmt"

// ValidateDeviceNamePattern validates that the device name follows expected NVMe naming patterns
// This function is exported for testing purposes
func ValidateDeviceNamePattern(deviceName string) error {
	return fmt.Errorf("NVMe operations are not supported on this platform")
}

func (u *Util) GetAllDevices() ([]DeviceFileAttributes, error) {
	return nil, fmt.Errorf("NVMe operations are not supported on this platform")
}

func (u *Util) GetDeviceSerial(*DeviceFileAttributes) (string, error) {
	return "", fmt.Errorf("NVMe operations are not supported on this platform")
}

func (u *Util) GetDeviceModel(*DeviceFileAttributes) (string, error) {
	return "", fmt.Errorf("NVMe operations are not supported on this platform")
}

func (u *Util) IsEbsDevice(*DeviceFileAttributes) (bool, error) {
	return false, fmt.Errorf("NVMe operations are not supported on this platform")
}

func (u *Util) IsInstanceStoreDevice(*DeviceFileAttributes) (bool, error) {
	return false, fmt.Errorf("NVMe operations are not supported on this platform")
}

func (u *Util) DetectDeviceType(*DeviceFileAttributes) (string, error) {
	return "", fmt.Errorf("NVMe operations are not supported on this platform")
}

func (u *Util) DevicePath(string) (string, error) {
	return "", fmt.Errorf("NVMe operations are not supported on this platform")
}
