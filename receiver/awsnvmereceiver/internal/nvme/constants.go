// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package nvme

const (
	devDirectoryPath = "/dev"

	nvmeDevicePrefix     = "nvme"
	nvmeSysDirectoryPath = "/sys/class/nvme"

	nvmeIoctlAdminCmd = 0xC0484E41
	logID             = 0xD0
)
