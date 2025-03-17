// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

type NvmeUtilInterface interface {
	GetAllDevices() ([]NvmeDeviceFileAttributes, error)
	GetDeviceSerial(device *NvmeDeviceFileAttributes) (string, error)
	GetDeviceModel(device *NvmeDeviceFileAttributes) (string, error)
	IsEbsDevice(device *NvmeDeviceFileAttributes) (bool, error)
}

type NvmeUtil struct {
}

