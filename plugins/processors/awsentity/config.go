// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	// ScrapeDatapointAttribute determines if the processor should scrape OTEL datapoint
	// attributes for entity related information. This option is mainly used for components
	// that emit all attributes to datapoint level instead of resource level. All telegraf
	// plugins have this behavior.
	ScrapeDatapointAttribute bool `mapstructure:"scrape_datapoint_attribute,omitempty"`
}

// Verify Config implements Processor interface.
var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	return nil
}
