// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"errors"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals/rules"
)

type Config struct {
	Resolvers []Resolver   `mapstructure:"resolvers"`
	Rules     []rules.Rule `mapstructure:"rules"`
}

func (cfg *Config) Validate() error {
	if len(cfg.Resolvers) == 0 {
		return errors.New("resolvers must not be empty")
	}
	for _, resolver := range cfg.Resolvers {
		switch resolver.Platform {
		case PlatformEKS:
			if resolver.Name == "" {
				return errors.New("name must not be empty for eks resolver")
			}
		case PlatformGeneric:
		default:
			return errors.New("unknown resolver")
		}
	}
	return nil
}
