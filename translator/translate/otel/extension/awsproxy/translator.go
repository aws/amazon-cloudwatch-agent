// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsproxy

import (
	"fmt"
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var (
	endpointOverrideKey = common.ConfigKey(common.TracesKey, common.EndpointOverrideKey)
	localModeKey        = common.ConfigKey(common.TracesKey, common.LocalModeKey)
)

type translator struct {
	name    string
	factory extension.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, awsproxy.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(common.TracesKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.TracesKey}
	}
	cfg := t.factory.CreateDefaultConfig().(*awsproxy.Config)
	cfg.ProxyConfig.CertificateFilePath = os.Getenv(envconfig.AWS_CA_BUNDLE)
	if conf.IsSet(endpointOverrideKey) {
		cfg.ProxyConfig.AWSEndpoint, _ = common.GetString(conf, endpointOverrideKey)
	}
	cfg.ProxyConfig.IMDSRetries = retryer.GetDefaultRetryNumber()
	if context.CurrentContext().Mode() == config.ModeOnPrem || context.CurrentContext().Mode() == config.ModeOnPremise {
		cfg.ProxyConfig.LocalMode = true
	}
	if localMode, ok := common.GetBool(conf, localModeKey); ok {
		cfg.ProxyConfig.LocalMode = localMode
	}
	if profileKey, ok := agent.Global_Config.Credentials[agent.Profile_Key]; ok {
		cfg.ProxyConfig.Profile = fmt.Sprintf("%v", profileKey)
	}
	cfg.ProxyConfig.Region = getRegion(conf)
	cfg.ProxyConfig.RoleARN = getRoleARN(conf)
	if credentialsFileKey, ok := agent.Global_Config.Credentials[agent.CredentialsFile_Key]; ok {
		cfg.ProxyConfig.SharedCredentialsFile = []string{fmt.Sprintf("%v", credentialsFileKey)}
	}
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
