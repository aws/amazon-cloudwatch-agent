// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

const (
	devDirectoryPath = "/dev" //nolint:unused // Used in Linux-specific code

	nvmeDevicePrefix     = "nvme"            //nolint:unused // Used in Linux-specific code
	nvmeSysDirectoryPath = "/sys/class/nvme" //nolint:unused // Used in Linux-specific code

	nvmeIoctlAdminCmd = 0xC0484E41 //nolint:unused // Used in Linux-specific code
	logID             = 0xD0       //nolint:unused // Used in Linux-specific code
)
