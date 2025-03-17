// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

type UtilInterface interface {
	GetAllDevices() ([]DeviceFileAttributes, error)
	GetDeviceSerial(device *DeviceFileAttributes) (string, error)
	GetDeviceModel(device *DeviceFileAttributes) (string, error)
	IsEbsDevice(device *DeviceFileAttributes) (bool, error)
}

type Util struct {
}
