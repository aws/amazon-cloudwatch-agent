// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package azuretagger

import (
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the azuretagger processor
type Config struct {
	// RefreshTagsInterval is the interval for refreshing Azure tags from IMDS.
	// Set to 0 to disable periodic refresh (tags fetched once at startup).
	// Default: 0 (no refresh after initial fetch succeeds)
	RefreshTagsInterval time.Duration `mapstructure:"refresh_tags_interval"`

	// AzureMetadataTags specifies which Azure metadata fields to add as dimensions.
	// Supported: "InstanceId", "InstanceType", "ImageId", "VMScaleSetName",
	//            "ResourceGroupName", "SubscriptionId"
	AzureMetadataTags []string `mapstructure:"azure_metadata_tags"`

	// AzureInstanceTagKeys specifies which Azure VM tags to add as dimensions.
	// Use ["*"] to include all tags.
	AzureInstanceTagKeys []string `mapstructure:"azure_instance_tag_keys"`
}

// Verify Config implements component.Config interface
var _ component.Config = (*Config)(nil)

// Validate validates the processor configuration
func (cfg *Config) Validate() error {
	return nil
}
