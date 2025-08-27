// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

type DeviceInfoProvider interface {
	GetAllDevices() ([]DeviceFileAttributes, error)
	GetDeviceSerial(*DeviceFileAttributes) (string, error)
	GetDeviceModel(*DeviceFileAttributes) (string, error)
	DevicePath(string) (string, error)
}

type Util struct {
}
