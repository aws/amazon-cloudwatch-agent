// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disktagger

import (
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/cloudprovider"
)

const (
	AttributeDiskID = "VolumeId"
)

type Config struct {
	// CloudProvider specifies which cloud's disk tagging to use (set at translation time).
	CloudProvider cloudprovider.CloudProvider `mapstructure:"-"`
	// InstanceID is the cloud instance ID (set at translation time for AWS).
	InstanceID string `mapstructure:"-"`
	// Region is the cloud region (set at translation time for AWS).
	Region string `mapstructure:"-"`
	// RefreshInterval is how often the disk-to-volume mapping is refreshed.
	// Set to 0 to disable periodic refresh after initial retrieval.
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
	// DiskDeviceTagKey is the metric attribute key that contains the device name
	// (e.g. "device" for telegraf disk metrics producing attributes like device=sda).
	DiskDeviceTagKey string `mapstructure:"disk_device_tag_key"`
}
