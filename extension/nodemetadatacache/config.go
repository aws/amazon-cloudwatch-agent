// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nodemetadatacache

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

type Config struct {
	Namespace string `mapstructure:"namespace"`
}

func (c *Config) Validate() error {
	if c.Namespace == "" {
		return errors.New("namespace must not be empty")
	}
	return nil
}

var _ component.Config = (*Config)(nil)
