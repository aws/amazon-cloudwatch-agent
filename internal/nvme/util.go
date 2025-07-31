// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

type DeviceInfoProvider interface {
	GetAllDevices() ([]DeviceFileAttributes, error)
	GetDeviceSerial(*DeviceFileAttributes) (string, error)
	GetDeviceModel(*DeviceFileAttributes) (string, error)
	IsEbsDevice(*DeviceFileAttributes) (bool, error)
	IsInstanceStoreDevice(*DeviceFileAttributes) (bool, error)
	DevicePath(string) (string, error)
}

type Util struct {
}
