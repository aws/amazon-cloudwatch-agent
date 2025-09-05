// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

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
	globallogs "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	logsutil "github.com/aws/amazon-cloudwatch-agent/translator/translate/util"
)

type translator struct {
	name           string
	factory        exporter.Factory
	collectConfig  map[string]interface{} // Specific journald collect_list entry config
}

var _ common.ComponentTranslator = (*translator)(nil)

var (
	roleARNPathKey      = common.ConfigKey(common.LogsKey, common.CredentialsKey, common.RoleARNKey)
	endpointOverrideKey = common.ConfigKey(common.LogsKey, common.EndpointOverrideKey)
)

func NewTranslator() common.ComponentTranslator {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{name, awscloudwatchlogsexporter.NewFactory(), nil}
}

func NewTranslatorWithConfig(name string, collectConfig map[string]interface{}) common.ComponentTranslator {
	return &translator{name, awscloudwatchlogsexporter.NewFactory(), collectConfig}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an awscloudwatchlogsexporter config specifically configured for journald logs
func (t *translator) Translate(c *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*awscloudwatchlogsexporter.Config)
	cfg.MiddlewareID = &agenthealth.LogsID

	// Configure from the specific collect_list entry
	if t.collectConfig != nil {
		if err := t.setJournaldFieldsFromConfig(cfg); err != nil {
			return nil, err
		}
	}

	// Set AWS session configuration
	cfg.AWSSessionSettings.CertificateFilePath = os.Getenv(envconfig.AWS_CA_BUNDLE)
	if endpoint, ok := common.GetString(c, endpointOverrideKey); ok {
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

func (t *translator) setJournaldFieldsFromConfig(cfg *awscloudwatchlogsexporter.Config) error {
	if t.collectConfig == nil {
		return nil
	}

	// Set log group name with placeholder resolution
	if logGroupName, ok := t.collectConfig[common.LogGroupName].(string); ok && logGroupName != "" {
		cfg.LogGroupName = logsutil.ResolvePlaceholder(logGroupName, globallogs.GlobalLogConfig.MetadataInfo)
	} else {
		// Default log group name if not specified
		cfg.LogGroupName = "journald-logs"
	}

	// Set log stream name with placeholder resolution
	if logStreamName, ok := t.collectConfig[common.LogStreamName].(string); ok && logStreamName != "" {
		cfg.LogStreamName = logsutil.ResolvePlaceholder(logStreamName, globallogs.GlobalLogConfig.MetadataInfo)
	} else {
		// Default log stream name if not specified
		cfg.LogStreamName = "{instance_id}"
	}

	// Set retention in days if available
	if retentionInDays, ok := t.collectConfig["retention_in_days"].(float64); ok {
		cfg.LogRetention = int64(retentionInDays)
	}

	return nil
}