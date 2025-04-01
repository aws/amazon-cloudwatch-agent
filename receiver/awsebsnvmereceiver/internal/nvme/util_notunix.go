// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !linux

package nvme

import "errors"

func (u *Util) GetAllDevices() ([]DeviceFileAttributes, error) {
	return nil, errors.New("nvme not supported")
}

func (u *Util) GetDeviceSerial(device *DeviceFileAttributes) (string, error) {
	return "", errors.New("nvme not supported")
}

func (u *Util) GetDeviceModel(device *DeviceFileAttributes) (string, error) {
	return "", errors.New("nvme not supported")
}

func (u *Util) IsEbsDevice(device *DeviceFileAttributes) (bool, error) {
	return false, errors.New("nvme not supported")
}

func DevicePath(device string) (string, error) {
  return "", errors.New("nvme not supported")
}
