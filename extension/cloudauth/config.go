// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration for the cloud auth extension.
type Config struct {
	// TokenFile is a path to a file containing an OIDC/JWT token.
	TokenFile string `mapstructure:"token_file,omitempty"`

	// STSResource is the audience/resource claim requested in the OIDC token.
	STSResource string `mapstructure:"sts_resource,omitempty"`
}

var _ component.Config = (*Config)(nil)

func (c *Config) Validate() error {
	return nil
}
