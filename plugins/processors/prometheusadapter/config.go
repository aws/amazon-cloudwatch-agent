// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheusadapter

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct{}

// Verify Config implements Processor interface.
var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	return nil
}
