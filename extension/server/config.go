// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package server

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	ListenAddress string `mapstructure:"listen_addr"`
	TLSCAPath     string `mapstructure:"tls_ca_path, omitempty"`
	TLSCertPath   string `mapstructure:"tls_cert_path, omitempty"`
	TLSKeyPath    string `mapstructure:"tls_key_path, omitempty"`
}

var _ component.Config = (*Config)(nil)
