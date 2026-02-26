// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disktagger

import "time"

const (
	AttributeDiskID = "VolumeId"
)

type Config struct {
	// RefreshInterval is how often the disk-to-volume mapping is refreshed.
	// Set to 0 to disable periodic refresh after initial retrieval.
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
	// DiskDeviceTagKey is the metric attribute key that contains the device name
	// (e.g. "device" for telegraf disk metrics producing attributes like device=sda).
	DiskDeviceTagKey string `mapstructure:"disk_device_tag_key"`
}
