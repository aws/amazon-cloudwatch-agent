// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"time"

	"go.opentelemetry.io/collector/component"
)

var SupportedAppendDimensions = map[string]string{
	"AutoScalingGroupName": "${aws:AutoScalingGroupName}",
	"ImageId":              "${aws:ImageId}",
	"InstanceId":           "${aws:InstanceId}",
	"InstanceType":         "${aws:InstanceType}",
}

const (
	AttributeVolumeId            = "VolumeId"
	ValueAppendDimensionVolumeId = "${aws:VolumeId}"
)

type Config struct {
	RefreshIntervalSeconds time.Duration `mapstructure:"refresh_interval_seconds"`
	EC2MetadataTags        []string      `mapstructure:"ec2_metadata_tags"`
	EC2InstanceTagKeys     []string      `mapstructure:"ec2_instance_tag_keys"`
	EBSDeviceKeys          []string      `mapstructure:"ebs_device_keys,omitempty"`

	//The tag key in the metrics for disk device
	DiskDeviceTagKey string `mapstructure:"disk_device_tag_key,omitempty"`

	// unlike other AWS plugins, this one determines the region from ec2 metadata not user configuration
	AccessKey   string `mapstructure:"access_key,omitempty"`
	SecretKey   string `mapstructure:"secret_key,omitempty"`
	RoleARN     string `mapstructure:"role_arn,omitempty"`
	Profile     string `mapstructure:"profile,omitempty"`
	Filename    string `mapstructure:"shared_credential_file,omitempty"`
	Token       string `mapstructure:"token,omitempty"`
	IMDSRetries int    `mapstructure:"imds_retries,omitempty"`
}

// Verify Config implements Processor interface.
var _ component.Config = (*Config)(nil)

// Validate does not check for unsupported dimension key-value pairs, because those
// get silently dropped and ignored during translation.
func (cfg *Config) Validate() error {
	return nil
}
