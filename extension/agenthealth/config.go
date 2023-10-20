// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import "go.opentelemetry.io/collector/component"

type Config struct {
	IsUsageDataEnabled bool `mapstructure:"is_usage_data_enabled"`
}

var _ component.Config = (*Config)(nil)
