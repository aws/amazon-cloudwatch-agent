// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// sigv4auth.go is retained as dead code to support future OTLP delivery mode.

package syslog

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const sigv4AuthSyslogName = "syslog"

type sigv4AuthTranslator struct {
	factory extension.Factory
	region  string
	roleARN string
}

var _ common.ComponentTranslator = (*sigv4AuthTranslator)(nil)

func newSigV4AuthTranslator(region, roleARN string) common.ComponentTranslator {
	return &sigv4AuthTranslator{
		factory: sigv4authextension.NewFactory(),
		region:  region,
		roleARN: roleARN,
	}
}

func (t *sigv4AuthTranslator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), sigv4AuthSyslogName)
}

func (t *sigv4AuthTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*sigv4authextension.Config)
	cfg.Region = t.region
	cfg.Service = "logs"
	if t.roleARN != "" {
		cfg.AssumeRole = sigv4authextension.AssumeRole{ARN: t.roleARN, STSRegion: t.region}
	}
	return cfg, nil
}
