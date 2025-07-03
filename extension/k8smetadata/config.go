// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8smetadata

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"
)

type Config struct {
	Objects []string `mapstructure:"objects"`
}

func (c *Config) Validate() error {
	var errs error
	if len(c.Objects) == 0 {
		errs = multierr.Append(errs, errors.New("no k8s objects passed in"))
	}

	allowedObjects := map[string]bool{
		"endpointslices": true,
		"services":       true,
	}

	for _, obj := range c.Objects {
		if !allowedObjects[obj] {
			errs = multierr.Append(errs, errors.New("invalid k8s object: "+obj+". Only 'endpointslices' and 'services' are allowed"))
		}
	}
	return errs
}

var _ component.Config = (*Config)(nil)
