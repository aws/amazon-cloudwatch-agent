// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsappsignals

import (
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsappsignals/customconfiguration"
)

type Config struct {
	Resolvers []string                   `mapstructure:"resolvers"`
	Rules     []customconfiguration.Rule `mapstructure:"rules"`
}

func (cfg *Config) Validate() error {
	// TODO: validate those mandatory fields (if exist) in the config
	return nil
}
