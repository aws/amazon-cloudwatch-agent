// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsebsnvmereceiver

import (
	"github.com/aws/amazon-cloudwatch-agent/receiver/awsebsnvmereceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
}

var _ component.Config = (*Config)(nil)

// TODO: validate the config...
func (c *Config) Validate() error {
	return nil
}
