// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !linux

package nvme

import "errors"

func (u *Util) GetAllDevices() ([]DeviceFileAttributes, error) {
	return nil, errors.New("nvme device discovery is only supported on Linux")
}

func (u *Util) GetDeviceSerial(device *DeviceFileAttributes) (string, error) {
	return "", errors.New("nvme device operations are only supported on Linux")
}

func (u *Util) GetDeviceModel(device *DeviceFileAttributes) (string, error) {
	return "", errors.New("nvme device operations are only supported on Linux")
}

func (u *Util) IsEbsDevice(device *DeviceFileAttributes) (bool, error) {
	return false, errors.New("nvme device operations are only supported on Linux")
}

func (u *Util) IsInstanceStoreDevice(device *DeviceFileAttributes) (bool, error) {
	return false, errors.New("nvme device operations are only supported on Linux")
}

func (u *Util) DevicePath(device string) (string, error) {
	return "", errors.New("nvme device operations are only supported on Linux")
}
