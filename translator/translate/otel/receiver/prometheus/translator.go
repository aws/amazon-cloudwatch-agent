// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/file"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

const (
	otelConfigParsingError = "has invalid keys: global"
	defaultTLSCaPath       = "/etc/amazon-cloudwatch-observability-agent-cert/tls-ca.crt"
	defaultTLSCertPath     = "/etc/amazon-cloudwatch-observability-agent-ta-client-cert/client.crt"
	defaultTLSKeyPath      = "/etc/amazon-cloudwatch-observability-agent-ta-client-cert/client.key"
)

type translator struct {
	name      string
	configKey string // config key to prometheus, e.g. logs.metrics_collected.prometheus
	factory   receiver.Factory
}

func WithConfigKey(configKey string) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.configKey = configKey
		}
	}
}

func WithName(name string) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.name = name
		}
	}
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslator(opts ...common.TranslatorOption) common.ComponentTranslator {
	t := &translator{factory: prometheusreceiver.NewFactory()}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*prometheusreceiver.Config)
	configPathKey := common.ConfigKey(t.configKey, common.PrometheusConfigPathKey)

	if !conf.IsSet(configPathKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configPathKey}
	}

	configPath, _ := common.GetString(conf, configPathKey)
	processedConfigPath, err := util.GetConfigPath("prometheus.yaml", configPathKey, configPath, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to process prometheus config with given config: %w", err)
	}
	configPath = processedConfigPath.(string)

	// Create default scrape config with file_sd_config
	fileSD := &file.SDConfig{
		Files:           []string{configPath},
	}

	scrapeConfig := &config.ScrapeConfig{
			ServiceDiscoveryConfigs: discovery.Configs{fileSD},
		}

	// Initialize PrometheusConfig if nil
	if cfg.PrometheusConfig == nil {
		cfg.PrometheusConfig = &prometheusreceiver.PromConfig{}
	}

	// Set the scrape config
	cfg.PrometheusConfig.ScrapeConfigs = []*config.ScrapeConfig{scrapeConfig}

	return cfg, nil
}