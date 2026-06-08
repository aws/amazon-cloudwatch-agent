// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslog

import (
	"fmt"
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
)

// deliveryModePLE and deliveryModeOTLP are retained to support future OTLP delivery mode.
const (
	deliveryModePLE  = "PutLogEvents"
	deliveryModeOTLP = "OTLP"
)

// newExporterTranslator is retained as dead code to support future OTLP delivery mode.
func newExporterTranslator(name, logGroupName, logStreamName string, retentionInDays int64, deliveryMode string, _ *confmap.Conf) common.ComponentTranslator {
	if deliveryMode == deliveryModeOTLP {
		return newOTLPExporterTranslator(name)
	}
	return newCWLExporterTranslator(name, logGroupName, logStreamName, retentionInDays)
}

// CWL (PutLogEvents) exporter

type cwlExporterTranslator struct {
	name            string
	logGroupName    string
	logStreamName   string
	retentionInDays int64
	factory         exporter.Factory
}

var _ common.ComponentTranslator = (*cwlExporterTranslator)(nil)

func newCWLExporterTranslator(name, logGroupName, logStreamName string, retentionInDays int64) common.ComponentTranslator {
	return &cwlExporterTranslator{
		name:            name,
		logGroupName:    logGroupName,
		logStreamName:   logStreamName,
		retentionInDays: retentionInDays,
		factory:         awscloudwatchlogsexporter.NewFactory(),
	}
}

func (t *cwlExporterTranslator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *cwlExporterTranslator) Translate(c *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*awscloudwatchlogsexporter.Config)
	cfg.MiddlewareID = &agenthealth.LogsID
	cfg.LogGroupName = t.logGroupName
	cfg.LogStreamName = t.logStreamName
	cfg.LogRetention = t.retentionInDays
	cfg.RawLog = true

	cfg.CertificateFilePath = os.Getenv(envconfig.AWS_CA_BUNDLE)
	endpointKey := common.ConfigKey(common.LogsKey, common.EndpointOverrideKey)
	if endpoint, ok := common.GetString(c, endpointKey); ok {
		cfg.Endpoint = endpoint
		cfg.AWSSessionSettings.Endpoint = endpoint
	}
	cfg.IMDSRetries = retryer.GetDefaultRetryNumber()
	if profileKey, ok := agent.Global_Config.Credentials[agent.Profile_Key]; ok {
		cfg.Profile = fmt.Sprintf("%v", profileKey)
	}
	cfg.Region = agent.Global_Config.Region
	cfg.RoleARN = agent.Global_Config.Role_arn
	roleARNKey := common.ConfigKey(common.LogsKey, common.CredentialsKey, common.RoleARNKey)
	if c.IsSet(roleARNKey) {
		cfg.RoleARN, _ = common.GetString(c, roleARNKey)
	}
	if credentialsFileKey, ok := agent.Global_Config.Credentials[agent.CredentialsFile_Key]; ok {
		cfg.SharedCredentialsFile = []string{fmt.Sprintf("%v", credentialsFileKey)}
	}
	if context.CurrentContext().Mode() == config.ModeOnPrem || context.CurrentContext().Mode() == config.ModeOnPremise {
		cfg.LocalMode = true
	}
	return cfg, nil
}

// OTLP exporter — retained as dead code to support future OTLP delivery mode.

type otlpExporterTranslator struct {
	name    string
	factory exporter.Factory
}

var _ common.ComponentTranslator = (*otlpExporterTranslator)(nil)

func newOTLPExporterTranslator(name string) common.ComponentTranslator {
	return &otlpExporterTranslator{
		name:    name,
		factory: otlphttpexporter.NewFactory(),
	}
}

func (t *otlpExporterTranslator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *otlpExporterTranslator) Translate(c *confmap.Conf) (component.Config, error) {
	region := agent.Global_Config.Region
	logsEndpoint := fmt.Sprintf("https://logs.%s.amazonaws.com/v1/logs", region)
	endpointKey := common.ConfigKey(common.LogsKey, common.EndpointOverrideKey)
	if ep, ok := common.GetString(c, endpointKey); ok {
		logsEndpoint = ep
	}

	// The auth chain is: otlphttp → headers_setter → provisioner → sigv4auth.
	// headers_setter injects x-aws-log-group/stream/retention headers.
	headersSetterID := component.NewIDWithName(component.MustNewType("headers_setter"), t.name)

	cfgMap := map[string]any{
		"logs_endpoint": logsEndpoint,
		"compression":   "gzip",
		"auth": map[string]any{
			"authenticator": headersSetterID.String(),
		},
	}

	cfg := t.factory.CreateDefaultConfig()
	if err := confmap.NewFromStringMap(cfgMap).Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
