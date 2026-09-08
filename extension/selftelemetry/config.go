// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package selftelemetry

import (
	"time"

	"go.opentelemetry.io/collector/component"
)

const defaultDiscoveryInterval = 10 * time.Second

type Config struct {
	// DiscoveryInterval is how often the registries are rescanned for metric families that appeared
	// after the last scan, since receivers register theirs well after extensions start.
	DiscoveryInterval time.Duration `mapstructure:"discovery_interval,omitempty"`
}

var _ component.Config = (*Config)(nil)

func (c *Config) discoveryInterval() time.Duration {
	if c.DiscoveryInterval <= 0 {
		return defaultDiscoveryInterval
	}
	return c.DiscoveryInterval
}
