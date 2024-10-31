// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/translator/util"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

const (
	otelConfigParsingError = "has invalid keys: global"
	defaultTlsCaPath       = "/etc/amazon-cloudwatch-observability-agent-cert/tls-ca.crt"
)

var (
	configPathKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, common.PrometheusKey, common.PrometheusConfigPathKey)
)

type translator struct {
	name    string
	factory receiver.Factory
}

type Option func(any)

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslator(opts ...Option) common.Translator[component.Config] {
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

	if conf.IsSet(configPathKey) {
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
			// then add the default cert for TargetAllocator
			if cfg.TargetAllocator != nil && len(cfg.TargetAllocator.CollectorID) > 0 {
				cfg.TargetAllocator.TLSSetting.Config.CAFile = defaultTlsCaPath
				cfg.TargetAllocator.TLSSetting.ReloadInterval = 10 * time.Second
			}
		}
	}

	return cfg, nil
}
