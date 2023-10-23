// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"go.opentelemetry.io/collector/component"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/client"
)

type Config struct {
	IsUsageDataEnabled bool               `mapstructure:"is_usage_data_enabled"`
	ClientStats        client.StatsConfig `mapstructure:"client_stats,omitempty"`
}

var _ component.Config = (*Config)(nil)
