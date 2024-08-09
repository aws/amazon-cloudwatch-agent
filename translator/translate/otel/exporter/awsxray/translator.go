// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsxray

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter"
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

const (
	concurrencyKey = "concurrency"
	resourceARNKey = "resource_arn"
)

type translator struct {
	name    string
	factory exporter.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

var (
	indexedAttributes = []string{
		"aws.local.service", "aws.local.operation", "aws.local.environment",
		"aws.remote.service", "aws.remote.operation", "aws.remote.environment",
		"aws.remote.resource.identifier", "aws.remote.resource.type",
	}
)

func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, awsxrayexporter.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

// Translate creates an exporter config based on the fields in the
// traces section of the JSON config.
// TODO: remove dependency on global config.
func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(common.TracesKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.TracesKey}
	}
	cfg := t.factory.CreateDefaultConfig().(*awsxrayexporter.Config)

	if isAppSignals(conf) {
		cfg.IndexedAttributes = indexedAttributes
	}

	c := confmap.NewFromStringMap(map[string]interface{}{
		"telemetry": map[string]interface{}{
			"enabled":          true,
			"include_metadata": true,
		},
	})
	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal into awsxrayexporter config: %w", err)
	}
	cfg.AWSSessionSettings.CertificateFilePath = os.Getenv(envconfig.AWS_CA_BUNDLE)
	if endpointOverride, ok := common.GetString(conf, common.ConfigKey(common.TracesKey, common.EndpointOverrideKey)); ok {
		cfg.AWSSessionSettings.Endpoint = endpointOverride
	}
	cfg.AWSSessionSettings.IMDSRetries = retryer.GetDefaultRetryNumber()
	if context.CurrentContext().Mode() == config.ModeOnPrem || context.CurrentContext().Mode() == config.ModeOnPremise {
		cfg.AWSSessionSettings.LocalMode = true
	}
	if localMode, ok := common.GetBool(conf, common.ConfigKey(common.TracesKey, common.LocalModeKey)); ok {
		cfg.AWSSessionSettings.LocalMode = localMode
	}
	if insecure, ok := common.GetBool(conf, common.ConfigKey(common.TracesKey, common.InsecureKey)); ok {
		cfg.AWSSessionSettings.NoVerifySSL = insecure
	}
	if concurrency, ok := common.GetNumber(conf, common.ConfigKey(common.TracesKey, concurrencyKey)); ok {
		cfg.AWSSessionSettings.NumberOfWorkers = int(concurrency)
	}
	if profileKey, ok := agent.Global_Config.Credentials[agent.Profile_Key]; ok {
		cfg.AWSSessionSettings.Profile = fmt.Sprintf("%v", profileKey)
	}
	if proxyAddress, ok := common.GetString(conf, common.ConfigKey(common.TracesKey, common.ProxyOverrideKey)); ok {
		cfg.AWSSessionSettings.ProxyAddress = proxyAddress
	}
	if resourceARN, ok := common.GetString(conf, common.ConfigKey(common.TracesKey, resourceARNKey)); ok {
		cfg.AWSSessionSettings.ResourceARN = resourceARN
	}
	cfg.AWSSessionSettings.Region = getRegion(conf)
	cfg.AWSSessionSettings.RoleARN = getRoleARN(conf)
	if credentialsFileKey, ok := agent.Global_Config.Credentials[agent.CredentialsFile_Key]; ok {
		cfg.AWSSessionSettings.SharedCredentialsFile = []string{fmt.Sprintf("%v", credentialsFileKey)}
	}
	cfg.MiddlewareID = &agenthealth.TracesID
	return cfg, nil
}

func getRoleARN(conf *confmap.Conf) string {
	key := common.ConfigKey(common.TracesKey, common.CredentialsKey, common.RoleARNKey)
	roleARN, ok := common.GetString(conf, key)
	if !ok {
		roleARN = agent.Global_Config.Role_arn
	}
	return roleARN
}

func getRegion(conf *confmap.Conf) string {
	key := common.ConfigKey(common.TracesKey, common.RegionOverrideKey)
	region, ok := common.GetString(conf, key)
	if !ok {
		region = agent.Global_Config.Region
	}
	return region
}

func isAppSignals(conf *confmap.Conf) bool {
	return conf.IsSet(common.AppSignalsTraces) || conf.IsSet(common.AppSignalsTracesFallback)
}
