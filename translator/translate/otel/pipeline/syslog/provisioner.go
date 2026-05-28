// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// provisioner.go is retained as dead code to support future OTLP delivery mode.

package syslog

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/awscloudwatchlogsprovisionerextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type provisionerTranslator struct {
	name         string
	factory      extension.Factory
	region       string
	logGroup     string
	logStream    string
	logRetention int64
}

var _ common.ComponentTranslator = (*provisionerTranslator)(nil)

func newProvisionerTranslator(name, region, logGroup, logStream string, logRetention int64) common.ComponentTranslator {
	return &provisionerTranslator{
		name:         name,
		factory:      awscloudwatchlogsprovisionerextension.NewFactory(),
		region:       region,
		logGroup:     logGroup,
		logStream:    logStream,
		logRetention: logRetention,
	}
}

func (t *provisionerTranslator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *provisionerTranslator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*awscloudwatchlogsprovisionerextension.Config)
	cfg.Region = t.region
	cfg.LogGroup = t.logGroup
	cfg.LogStream = t.logStream
	cfg.LogRetention = t.logRetention
	sigv4ID := component.NewIDWithName(component.MustNewType("sigv4auth"), sigv4AuthSyslogName)
	cfg.AdditionalAuth = &sigv4ID
	return cfg, nil
}
