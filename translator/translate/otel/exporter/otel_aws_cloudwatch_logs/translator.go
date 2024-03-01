// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otel_aws_cloudwatch_logs

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
)

//go:embed aws_cloudwatch_logs_default.yaml
var defaultAwsCloudwatchLogsDefault string

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

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, awscloudwatchlogsexporter.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an awscloudwatchlogsexporter exporter config based on the input json config
func (t *translator) Translate(c *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*awscloudwatchlogsexporter.Config)
	cfg.MiddlewareID = &agenthealth.LogsID

	var defaultConfig string
	// Add more else if when otel supports log reading
	if t.name == common.PipelineNameEmfLogs && t.isEmf(c) {
		defaultConfig = defaultAwsCloudwatchLogsDefault
	}

	if defaultConfig != "" {
		var rawConf map[string]interface{}
		if err := yaml.Unmarshal([]byte(defaultConfig), &rawConf); err != nil {
			return nil, fmt.Errorf("unable to read default config: %w", err)
		}
		conf := confmap.NewFromStringMap(rawConf)
		if err := conf.Unmarshal(&cfg); err != nil {
			return nil, fmt.Errorf("unable to unmarshal config: %w", err)
		}
	}

	// Add more else if when otel supports log reading
	if t.name == common.PipelineNameEmfLogs && t.isEmf(c) {
		if err := t.setEmfFields(c, cfg); err != nil {
			return nil, err
		}
	}

	cfg.AWSSessionSettings.CertificateFilePath = os.Getenv(envconfig.AWS_CA_BUNDLE)
	if c.IsSet(endpointOverrideKey) {
		cfg.AWSSessionSettings.Endpoint, _ = common.GetString(c, endpointOverrideKey)
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

	if conf.IsSet(streamNameKey) {
		cfg.LogStreamName = fmt.Sprintf("%v", conf.Get(streamNameKey))
	} else {
		rule := logs.LogStreamName{}
		_, val := rule.ApplyRule(conf.Get(common.LogsKey))
		if logStreamName, ok := val.(map[string]interface{})[common.LogStreamName]; !ok {
			return &common.MissingKeyError{ID: t.ID(), JsonKey: streamNameKey}
		} else {
			cfg.LogStreamName = logStreamName.(string)
		}
	}

	cfg.EmfOnly = true
	return nil
}
