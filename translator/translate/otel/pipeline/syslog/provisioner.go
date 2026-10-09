// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// provisioner.go is retained as dead code to support future OTLP delivery mode.
// It produces a provisioner extension (region + sigv4 auth) and a headers_setter
// extension that injects log group/stream/retention as HTTP headers, following
// the same pattern as Application Signals logs.

package syslog

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/awscloudwatchlogsprovisionerextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/headerssetter"
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
	sigv4ID := component.NewIDWithName(component.MustNewType("sigv4auth"), sigv4AuthSyslogName)
	cfg.AdditionalAuth = &sigv4ID
	return cfg, nil
}

// newHeadersSetterTranslator creates a headers_setter extension that injects
// log group/stream/retention as HTTP headers. The headers_setter chains to the
// provisioner, which chains to sigv4auth, forming the full auth chain:
//
//	otlphttp exporter → headers_setter → provisioner → sigv4auth
func newHeadersSetterTranslator(name string, provisionerID component.ID, logGroup, logStream string, logRetention int64) common.ComponentTranslator {
	headers := []headerssetter.HeaderMapping{
		{HeaderName: "x-aws-log-group", Value: logGroup},
		{HeaderName: "x-aws-log-stream", Value: logStream},
	}
	if logRetention > 0 {
		headers = append(headers, headerssetter.HeaderMapping{
			HeaderName: "x-aws-log-retention",
			Value:      fmt.Sprintf("%d", logRetention),
		})
	}
	return headerssetter.NewTranslatorWithName(name,
		headerssetter.WithAdditionalAuth(provisionerID),
		headerssetter.WithHeaders(headers),
	)
}
