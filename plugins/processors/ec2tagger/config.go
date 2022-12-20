// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ec2tagger

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
)

var SupportedAppendDimensions = map[string]string{
	"AutoScalingGroupName": "${aws:AutoScalingGroupName}",
	"ImageId":              "${aws:ImageId}",
	"InstanceId":           "${aws:InstanceId}",
	"InstanceType":         "${aws:InstanceType}",
}

type Config struct {
	config.ProcessorSettings `mapstructure:",squash"`

	RefreshIntervalSeconds time.Duration `mapstructure:"refresh_interval_seconds"`
	EC2MetadataTags        []string      `mapstructure:"ec2_metadata_tags"`
	EC2InstanceTagKeys     []string      `mapstructure:"ec2_instance_tag_keys"`
	EBSDeviceKeys          []string      `mapstructure:"ebs_device_keys,omitempty"`

	//The tag key in the metrics for disk device
	DiskDeviceTagKey string `mapstructure:"disk_device_tag_key,omitempty"`

	// unlike other AWS plugins, this one determines the region from ec2 metadata not user configuration
	AccessKey string `mapstructure:"access_key,omitempty"`
	SecretKey string `mapstructure:"secret_key,omitempty"`
	RoleARN   string `mapstructure:"role_arn,omitempty"`
	Profile   string `mapstructure:"profile,omitempty"`
	Filename  string `mapstructure:"shared_credential_file,omitempty"`
	Token     string `mapstructure:"token,omitempty"`
}

// Verify Config implements Processor interface.
var _ component.Config = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	if len(cfg.EC2MetadataTags) == 0 && len(cfg.EC2InstanceTagKeys) == 0 {
		return fmt.Errorf("append_dimensions set without any supported key-value pairs")
	}
	for _, t := range cfg.EC2MetadataTags {
		if _, ok := SupportedAppendDimensions[t]; !ok {
			return fmt.Errorf("Unsupported Dimension: %s", t)
		}
	}
	for _, k := range cfg.EC2InstanceTagKeys {
		if _, ok := SupportedAppendDimensions[k]; !ok {
			return fmt.Errorf("Unsupported Dimension: %s", k)
		}
	}
	return nil
}
