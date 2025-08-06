// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !linux

package nvme

import "errors"

func (u *Util) GetAllDevices() ([]DeviceFileAttributes, error) {
	return nil, errors.New("nvme not supported")
}

func (u *Util) GetDeviceSerial(_ *DeviceFileAttributes) (string, error) {
	return "", errors.New("nvme not supported")
}

func (u *Util) GetDeviceModel(_ *DeviceFileAttributes) (string, error) {
	return "", errors.New("nvme not supported")
}

func (u *Util) IsEbsDevice(_ *DeviceFileAttributes) (bool, error) {
	return false, errors.New("nvme not supported")
}

func (u *Util) IsInstanceStoreDevice(_ *DeviceFileAttributes) (bool, error) {
	return false, errors.New("nvme not supported")
}

func (u *Util) DevicePath(_ string) (string, error) {
	return "", errors.New("nvme not supported")
}
