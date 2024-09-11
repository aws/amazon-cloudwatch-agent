// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package server

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	ListenAddress string `mapstructure:"listen_addr"`
}

var _ component.Config = (*Config)(nil)
