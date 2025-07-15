// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
	"gopkg.in/yaml.v3"

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
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read prometheus config from path: %w", err)
	}

	escapedContent, err := escapePrometheusConfig(content)
	if err != nil {
		return nil, fmt.Errorf("unable to escape prometheus config: %w", err)
	}
	content = escapedContent

	var stringMap map[string]interface{}
	err = yaml.Unmarshal(content, &stringMap)
	if err != nil {
		return nil, err
	}
	componentParser := confmap.NewFromStringMap(stringMap)
	if componentParser == nil {
		return nil, fmt.Errorf("unable to parse config from filename %s", configPath)
	}
	err = componentParser.Unmarshal(&cfg)
	if err != nil {
		// passed in prometheus config is in plain prometheus format and not otel wrapper
		if !strings.Contains(err.Error(), otelConfigParsingError) {
			return nil, fmt.Errorf("unable to unmarshall config to otel prometheus config from filename %s", configPath)
		}

		var promCfg prometheusreceiver.PromConfig
		err = componentParser.Unmarshal(&promCfg)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshall config to prometheus config from filename %s", configPath)
		}
		cfg.PrometheusConfig.GlobalConfig = promCfg.GlobalConfig
		cfg.PrometheusConfig.ScrapeConfigs = promCfg.ScrapeConfigs
		cfg.PrometheusConfig.TracingConfig = promCfg.TracingConfig
	} else {
		// given prometheus config is in otel format so check if target allocator is being used
		// then add the default ca, cert, and key for TargetAllocator
		if cfg.TargetAllocator != nil && len(cfg.TargetAllocator.CollectorID) > 0 {
			cfg.TargetAllocator.TLSSetting.Config.CAFile = defaultTLSCaPath
			cfg.TargetAllocator.TLSSetting.Config.CertFile = defaultTLSCertPath
			cfg.TargetAllocator.TLSSetting.Config.KeyFile = defaultTLSKeyPath
			cfg.TargetAllocator.TLSSetting.ReloadInterval = 10 * time.Second
		}
	}

	return cfg, nil
}

func escapePrometheusConfig(content []byte) ([]byte, error) {
	var config map[any]any
	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, err
	}
	escapeStrings(config)
	return yaml.Marshal(config)
}

func escapeStrings(node any) {
	switch n := node.(type) {
	case map[any]any:
		for k, v := range n {
			if key, ok := k.(string); ok && key == "replacement" {
				if str, ok := v.(string); ok {
					n[k] = strings.ReplaceAll(str, "$", "$$$$")
				}
			}
			escapeStrings(v)
		}
	case map[string]interface{}:
		for k, v := range n {
			if k == "replacement" {
				if str, ok := v.(string); ok {
					n[k] = strings.ReplaceAll(str, "$", "$$$$")
				}
			}
			escapeStrings(v)
		}
	case []any:
		for _, v := range n {
			escapeStrings(v)
		}
	}
}
