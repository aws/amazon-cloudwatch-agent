// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package nvme

import (
	"fmt"
	"os"
	"strings"
)

// For unit testing
var osReadFile = os.ReadFile
var osReadDir = os.ReadDir

func (u *Util) GetAllDevices() ([]DeviceFileAttributes, error) {
	entries, err := osReadDir(DevDirectoryPath)
	if err != nil {
		return nil, err
	}

	devices := []DeviceFileAttributes{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), NvmeDevicePrefix) {
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
	data, err := osReadFile(fmt.Sprintf("%s/%s/serial", NvmeSysDirectoryPath, deviceName))
	if err != nil {
		return "", err
	}
	return cleanupString(string(data)), nil
}

func (u *Util) GetDeviceModel(device *DeviceFileAttributes) (string, error) {
	deviceName, err := device.BaseDeviceName()
	if err != nil {
		return "", err
	}
	data, err := osReadFile(fmt.Sprintf("%s/%s/model", NvmeSysDirectoryPath, deviceName))
	if err != nil {
		return "", err
	}
	return cleanupString(string(data)), nil
}

func (u *Util) IsEbsDevice(device *DeviceFileAttributes) (bool, error) {
	model, err := u.GetDeviceModel(device)
	if err != nil {
		return false, err
	}
	return model == EbsNvmeModelName, nil
}

func cleanupString(input string) string {
	// Some device info strings use fixed-width padding and/or end with a new line
	return strings.TrimSpace(strings.TrimSuffix(input, "\n"))
}
