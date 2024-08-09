// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsxray

import (
	"fmt"
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	bindAddressKey = "bind_address"
	tcpProxyKey    = "tcp_proxy"

	defaultEndpoint = "127.0.0.1:2000"
)

var (
	baseKey = common.ConfigKey(common.TracesKey, common.TracesCollectedKey, common.XrayKey)
)

type translator struct {
	name    string
	factory receiver.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator() common.Translator[component.Config] {
	return NewTranslatorWithName("")
}

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, awsxrayreceiver.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	if conf == nil || !conf.IsSet(baseKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: baseKey}
	}
	cfg := t.factory.CreateDefaultConfig().(*awsxrayreceiver.Config)
	cfg.Endpoint = defaultEndpoint
	cfg.ProxyServer.Endpoint = defaultEndpoint
	cfg.ProxyServer.RoleARN = getRoleARN(conf)
	cfg.ProxyServer.Region = getRegion(conf)
	if endpoint, ok := common.GetString(conf, common.ConfigKey(baseKey, bindAddressKey)); ok {
		cfg.Endpoint = endpoint
	}
	if endpoint, ok := common.GetString(conf, common.ConfigKey(baseKey, tcpProxyKey, bindAddressKey)); ok {
		cfg.ProxyServer.Endpoint = endpoint
	}
	if insecure, ok := common.GetBool(conf, common.ConfigKey(common.TracesKey, common.InsecureKey)); ok {
		cfg.ProxyServer.TLSSetting.Insecure = insecure
	}
	if context.CurrentContext().Mode() == config.ModeOnPrem || context.CurrentContext().Mode() == config.ModeOnPremise {
		cfg.ProxyServer.LocalMode = true
	}
	if localMode, ok := common.GetBool(conf, common.ConfigKey(common.TracesKey, common.LocalModeKey)); ok {
		cfg.ProxyServer.LocalMode = localMode
	}
	if profileKey, ok := agent.Global_Config.Credentials[agent.Profile_Key]; ok {
		cfg.ProxyServer.Profile = fmt.Sprintf("%v", profileKey)
	}
	if endpoint, ok := common.GetString(conf, common.ConfigKey(common.TracesKey, common.EndpointOverrideKey)); ok {
		cfg.ProxyServer.AWSEndpoint = endpoint
	}
	if proxyAddress, ok := common.GetString(conf, common.ConfigKey(common.TracesKey, common.ProxyOverrideKey)); ok {
		cfg.ProxyServer.ProxyAddress = proxyAddress
	}
	if credentialsFileKey, ok := agent.Global_Config.Credentials[agent.CredentialsFile_Key]; ok {
		cfg.ProxyServer.SharedCredentialsFile = []string{fmt.Sprintf("%v", credentialsFileKey)}
	}
	cfg.ProxyServer.CertificateFilePath = os.Getenv(envconfig.AWS_CA_BUNDLE)
	cfg.ProxyServer.IMDSRetries = retryer.GetDefaultRetryNumber()
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
