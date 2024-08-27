// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	Mode     string `mapstructure:"mode"`
	Region   string `mapstructure:"region"`
	Profile  string `mapstructure:"profile,omitempty"`
	RoleARN  string `mapstructure:"role_arn,omitempty"`
	Filename string `mapstructure:"shared_credential_file,omitempty"`
}

var _ component.Config = (*Config)(nil)
