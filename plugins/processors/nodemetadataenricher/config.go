// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nodemetadataenricher

import (
	"go.opentelemetry.io/collector/component"
)

// Config is the configuration for the nodemetadataenricher processor.
// It has no configuration fields — the processor obtains its cache reference
// from the nodemetadatacache extension singleton.
type Config struct{}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	return nil
}
