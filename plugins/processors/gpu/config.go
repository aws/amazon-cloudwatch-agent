// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package gpu

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	DropOriginalMetrics bool `mapstructure:"drop_original_metrics"`
}

// Verify Config implements Processor interface.
var _ component.Config = (*Config)(nil)

// Validate does not check for unsupported dimension key-value pairs, because those
// get silently dropped and ignored during translation.
func (cfg *Config) Validate() error {
	return nil
}
