// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !linux

package nvme

import "errors"

func (u *NvmeUtil) GetAllDevices() ([]NvmeDeviceFileAttributes, error) {
	return nil, errors.New("nvme stub: nvme not supported")
}

func (u *NvmeUtil) GetDeviceSerial(device *NvmeDeviceFileAttributes) (string, error) {
	return "", errors.New("nvme stub: nvme not supported")
}

func (u *NvmeUtil) GetDeviceModel(device *NvmeDeviceFileAttributes) (string, error) {
	return "", errors.New("nvme stub: nvme not supported")
}

func (u *NvmeUtil) IsEbsDevice(device *NvmeDeviceFileAttributes) (bool, error) {
	return false, errors.New("nvme stub: nvme not supported")
}
