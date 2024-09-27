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
	// ClusterName can be used to explicitly provide the Cluster's Name for scenarios where it's not
	// possible to auto-detect it using EC2 tags.
	ClusterName string `mapstructure:"cluster_name,omitempty"`
	// KubernetesMode
	KubernetesMode string `mapstructure:"kubernetes_mode,omitempty"`
	// Specific Mode agent is running on (i.e. EC2, EKS, ECS etc)
	Platform string `mapstructure:"platform,omitempty"`
	// EntityType determines the type of entity processing done for
	// telemetry. Possible values are Service and Resource
	EntityType string `mapstructure:"entity_type,omitempty"`
}

// Verify Config implements Processor interface.
var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	return nil
}
