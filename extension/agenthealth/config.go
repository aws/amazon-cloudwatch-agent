// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"go.opentelemetry.io/collector/component"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

type Config struct {
	IsUsageDataEnabled bool              `mapstructure:"is_usage_data_enabled"`
	Stats              agent.StatsConfig `mapstructure:"stats"`
}

var _ component.Config = (*Config)(nil)
