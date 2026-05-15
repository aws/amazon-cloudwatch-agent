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

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
)

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

	cfg.AWSSessionSettings.CertificateFilePath = os.Getenv(envconfig.AWS_CA_BUNDLE)
	endpointKey := common.ConfigKey(common.LogsKey, common.EndpointOverrideKey)
	if endpoint, ok := common.GetString(c, endpointKey); ok {
		cfg.Endpoint = endpoint
		cfg.AWSSessionSettings.Endpoint = endpoint
	}
	cfg.AWSSessionSettings.IMDSRetries = retryer.GetDefaultRetryNumber()
	if profileKey, ok := agent.Global_Config.Credentials[agent.Profile_Key]; ok {
		cfg.AWSSessionSettings.Profile = fmt.Sprintf("%v", profileKey)
	}
	cfg.AWSSessionSettings.Region = agent.Global_Config.Region
	cfg.AWSSessionSettings.RoleARN = agent.Global_Config.Role_arn
	roleARNKey := common.ConfigKey(common.LogsKey, common.CredentialsKey, common.RoleARNKey)
	if c.IsSet(roleARNKey) {
		cfg.AWSSessionSettings.RoleARN, _ = common.GetString(c, roleARNKey)
	}
	if credentialsFileKey, ok := agent.Global_Config.Credentials[agent.CredentialsFile_Key]; ok {
		cfg.AWSSessionSettings.SharedCredentialsFile = []string{fmt.Sprintf("%v", credentialsFileKey)}
	}
	if context.CurrentContext().Mode() == config.ModeOnPrem || context.CurrentContext().Mode() == config.ModeOnPremise {
		cfg.AWSSessionSettings.LocalMode = true
	}
	return cfg, nil
}
