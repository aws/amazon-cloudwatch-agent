// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

var errMissingRoleARN = errors.New("role_arn is required for cloud auth")

// Config defines the configuration for the cloud auth extension.
type Config struct {
	// RoleARN is the AWS IAM role to assume via AssumeRoleWithWebIdentity.
	RoleARN string `mapstructure:"role_arn"`

	// Region is the AWS region for STS calls. Populated by the translator
	// from the agent's global config; not user-facing.
	Region string `mapstructure:"region,omitempty"`

	// TokenFile is a path to a file containing an OIDC/JWT token. When set,
	// the extension reads the token from this file instead of auto-detecting
	// a cloud provider. The user is responsible for keeping the file current.
	TokenFile string `mapstructure:"token_file,omitempty"`

	// STSResource is the audience/resource claim requested in the OIDC token.
	// Defaults to "https://management.azure.com/" for Azure auto-detection.
	STSResource string `mapstructure:"sts_resource,omitempty"`
}

var _ component.Config = (*Config)(nil)

func (c *Config) Validate() error {
	if c.RoleARN == "" {
		return errMissingRoleARN
	}
	return nil
}
