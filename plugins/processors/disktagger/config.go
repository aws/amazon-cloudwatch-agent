// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package disktagger

import (
	"time"

	"go.opentelemetry.io/collector/component"
)

const (
	AttributeDiskID = "VolumeId"
)

type Config struct {
	RefreshInterval  time.Duration `mapstructure:"refresh_interval"`
	DiskDeviceTagKey string        `mapstructure:"disk_device_tag_key"`
	MiddlewareID     *component.ID `mapstructure:"middleware,omitempty"`
}
