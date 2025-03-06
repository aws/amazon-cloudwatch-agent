// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscloudwatchlogs

import (
	_ "embed"
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
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
)

const (
	defaultLogGroupName = "emf/logs/default"
)

var (
	emfBasePathKey      = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.Emf)
	roleARNPathKey      = common.ConfigKey(common.LogsKey, common.CredentialsKey, common.RoleARNKey)
	endpointOverrideKey = common.ConfigKey(common.LogsKey, common.EndpointOverrideKey)
	streamNameKey       = common.ConfigKey(common.LogsKey, common.LogStreamName)
)

type translator struct {
	name    string
	factory exporter.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{name, awscloudwatchlogsexporter.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an awscloudwatchlogsexporter exporter config based on the input json config
func (t *translator) Translate(c *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*awscloudwatchlogsexporter.Config)
	cfg.MiddlewareID = &agenthealth.LogsID

	// Add more else if when otel supports log reading
	if t.name == common.PipelineNameEmfLogs && t.isEmf(c) {
		if err := t.setEmfFields(c, cfg); err != nil {
			return nil, err
		}
	}

	cfg.AWSSessionSettings.CertificateFilePath = os.Getenv(envconfig.AWS_CA_BUNDLE)
	if endpoint, ok := common.GetString(c, endpointOverrideKey); ok {
		// for some reason the exporter has an endpoint field in the config that
		// clashes with the AWSSessionsSettings
		cfg.Endpoint = endpoint
		cfg.AWSSessionSettings.Endpoint = endpoint
	}
	cfg.AWSSessionSettings.IMDSRetries = retryer.GetDefaultRetryNumber()
	if profileKey, ok := agent.Global_Config.Credentials[agent.Profile_Key]; ok {
		cfg.AWSSessionSettings.Profile = fmt.Sprintf("%v", profileKey)
	}
	cfg.AWSSessionSettings.Region = agent.Global_Config.Region
	cfg.AWSSessionSettings.RoleARN = agent.Global_Config.Role_arn
	if c.IsSet(roleARNPathKey) {
		cfg.AWSSessionSettings.RoleARN, _ = common.GetString(c, roleARNPathKey)
	}
	if credentialsFileKey, ok := agent.Global_Config.Credentials[agent.CredentialsFile_Key]; ok {
		cfg.AWSSessionSettings.SharedCredentialsFile = []string{fmt.Sprintf("%v", credentialsFileKey)}
	}
	if context.CurrentContext().Mode() == config.ModeOnPrem || context.CurrentContext().Mode() == config.ModeOnPremise {
		cfg.AWSSessionSettings.LocalMode = true
	}
	return cfg, nil
}

func (t *translator) isEmf(conf *confmap.Conf) bool {
	return conf.IsSet(emfBasePathKey)
}

func (t *translator) setEmfFields(conf *confmap.Conf, cfg *awscloudwatchlogsexporter.Config) error {
	cfg.Region = agent.Global_Config.Region
	cfg.EmfOnly = true
	cfg.RawLog = true
	cfg.LogGroupName = defaultLogGroupName

	rule := logs.LogStreamName{}
	_, val := rule.ApplyRule(conf.Get(common.LogsKey))
	if logStreamName, ok := val.(map[string]any)[common.LogStreamName]; !ok {
		return &common.MissingKeyError{ID: t.ID(), JsonKey: streamNameKey}
	} else {
		cfg.LogStreamName = logStreamName.(string)
	}
	return nil
}
