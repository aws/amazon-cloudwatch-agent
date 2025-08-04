// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package nvme

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// For unit testing
var osReadFile = os.ReadFile
var osReadDir = os.ReadDir

func (u *Util) GetAllDevices() ([]DeviceFileAttributes, error) {
	entries, err := osReadDir(devDirectoryPath)
	if err != nil {
		return nil, err
	}

	devices := []DeviceFileAttributes{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), nvmeDevicePrefix) {
			device, err := ParseNvmeDeviceFileName(entry.Name())
			if err == nil {
				devices = append(devices, device)
			}
		}
	}

	return devices, nil
}

func (u *Util) GetDeviceSerial(device *DeviceFileAttributes) (string, error) {
	deviceName, err := device.BaseDeviceName()
	if err != nil {
		return "", err
	}
	data, err := osReadFile(fmt.Sprintf("%s/%s/serial", nvmeSysDirectoryPath, deviceName))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (u *Util) GetDeviceModel(device *DeviceFileAttributes) (string, error) {
	deviceName, err := device.BaseDeviceName()
	if err != nil {
		return "", err
	}
	data, err := osReadFile(fmt.Sprintf("%s/%s/model", nvmeSysDirectoryPath, deviceName))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (u *Util) IsEbsDevice(device *DeviceFileAttributes) (bool, error) {
	model, err := u.GetDeviceModel(device)
	if err != nil {
		return false, err
	}
	return model == ebsNvmeModelName, nil
}

func (u *Util) IsInstanceStoreDevice(device *DeviceFileAttributes) (bool, error) {
	model, err := u.GetDeviceModel(device)
	if err != nil {
		return false, err
	}
	return model == instanceStoreNvmeModelName, nil
}

func (u *Util) DevicePath(device string) (string, error) {
	return filepath.Join(devDirectoryPath, device), nil
}
