// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otel_aws_cloudwatch_logs

import (
	_ "embed"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

//go:embed aws_cloudwatch_logs_default.yaml
var defaultAwsCloudwatchLogsDefault string

var (
	emfBasePathKey = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.Emf)
	roleArnPathKey = common.ConfigKey(common.LogsKey, common.CredentialsKey, common.RoleARNKey)
	regionKey      = common.ConfigKey(common.AgentKey, common.Region)
	streamNameKey  = common.ConfigKey(common.LogsKey, common.LogStreamName)
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

	var defaultConfig string
	// Add more else if when otel supports log reading
	if t.name == common.PipelineNameEmfLogs && t.isEmf(c) {
		defaultConfig = defaultAwsCloudwatchLogsDefault
	} else {
		return cfg, nil
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

	if profile, ok := agent.Global_Config.Credentials[agent.Profile_Key]; ok {
		cfg.AWSSessionSettings.Profile = fmt.Sprintf("%v", profile)
		cfg.AWSSessionSettings.SharedCredentialsFile = []string{fmt.Sprintf("%v", agent.Global_Config.Credentials[agent.CredentialsFile_Key])}
	}
	cfg.AWSSessionSettings.RoleARN = agent.Global_Config.Role_arn
	if c.IsSet(roleArnPathKey) {
		cfg.AWSSessionSettings.RoleARN, _ = common.GetString(c, roleArnPathKey)
	}

	return cfg, nil
}

func (t *translator) isEmf(conf *confmap.Conf) bool {
	return conf.IsSet(emfBasePathKey)
}

func (t *translator) setEmfFields(conf *confmap.Conf, cfg *awscloudwatchlogsexporter.Config) error {
	if conf.IsSet(regionKey) {
		cfg.Region = fmt.Sprintf("%v", conf.Get(regionKey))
	}
	if conf.IsSet(streamNameKey) {
		cfg.LogStreamName = fmt.Sprintf("%v", conf.Get(streamNameKey))
	}
	// @TODO add support for containers
	metadata := util.GetMetadataInfo(util.Ec2MetadataInfoProvider)
	cfg.LogStreamName = util.ResolvePlaceholder(cfg.LogStreamName, metadata)
	return nil
}
