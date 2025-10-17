// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"fmt"

	"go.opentelemetry.io/collector/component"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/metadata"
)

type Config struct {
	IsUsageDataEnabled  bool                `mapstructure:"is_usage_data_enabled"`
	Stats               *agent.StatsConfig  `mapstructure:"stats,omitempty"`
	IsStatusCodeEnabled bool                `mapstructure:"is_status_code_enabled,omitempty"`
	UsageMetadata       []metadata.Metadata `mapstructure:"usage_metadata,omitempty"`
}

var _ component.Config = (*Config)(nil)

func (c *Config) Validate() error {
	for _, m := range c.UsageMetadata {
		if !metadata.IsSupported(m) {
			return fmt.Errorf("usage metadata %q is not supported", m)
		}
	}
	return nil
}
