// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

const (
	devDirectoryPath = "/dev"

	nvmeDevicePrefix     = "nvme"
	nvmeSysDirectoryPath = "/sys/class/nvme"

	ebsNvmeModelName           = "Amazon Elastic Block Store"
	instanceStoreNvmeModelName = "Amazon EC2 NVMe Instance Storage"

	nvmeIoctlAdminCmd  = 0xC0484E41
	instanceStoreMagic = 0xEC2C0D7E
	ebsMagic           = 0x3C23B510
	logID              = 0xD0
)
