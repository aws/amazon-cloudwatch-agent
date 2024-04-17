// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package deltatosparseprocessor

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	// Include specifies a filter on the metrics that should be converted.
	Include []string `mapstructure:"include"`
}

// Verify Config implements Processor interface.
var _ component.Config = (*Config)(nil)

func (config *Config) Validate() error {
	return nil
}
