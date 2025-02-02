// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package server

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/extension/server"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	defaultListenAddr     = ":4311"
	tlsServerCertFilePath = "/etc/amazon-cloudwatch-observability-agent-server-cert/server.crt"
	tlsServerKeyFilePath  = "/etc/amazon-cloudwatch-observability-agent-server-cert/server.key"
	caFilePath            = "/etc/amazon-cloudwatch-observability-agent-client-cert/tls-ca.crt"
)

type translator struct {
	name    string
	factory extension.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator() common.ComponentTranslator {
	return &translator{
		factory: server.NewFactory(),
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an extension configuration.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*server.Config)
	cfg.ListenAddress = defaultListenAddr
	cfg.TLSCAPath = caFilePath
	cfg.TLSCertPath = tlsServerCertFilePath
	cfg.TLSKeyPath = tlsServerKeyFilePath
	return cfg, nil
}
