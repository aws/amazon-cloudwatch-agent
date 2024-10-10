// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"crypto/tls"
	"fmt"
	"os"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	promconfig "github.com/prometheus/prometheus/config"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

var (
	configPathKey = common.ConfigKey(common.MetricsPrometheus, "prometheus_config_path")
)

type prometheusConfig struct {
	promconfig.Config
	//TargetAllocator targetAllocator `yaml:"target_allocator"`
}

type targetAllocator struct {
	Interval         time.Duration                            `yaml:"interval"`
	CollectorID      string                                   `yaml:"collector_id"`
	HTTPSDConfig     *prometheusreceiver.PromHTTPSDConfig     `yaml:"http_sd_config"`
	HTTPScrapeConfig *prometheusreceiver.PromHTTPClientConfig `yaml:"http_scrape_config"`
}

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
		// first unmarshall passed in prometheus config yaml into PromConfig
		promCfg := &prometheusConfig{}
		content, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read prometheus config from path: %w", err)
		}
		if err := yaml.Unmarshal(content, &promCfg); err != nil {
			return nil, fmt.Errorf("unable to unmarshall prometheus config yaml: %w", err)
		}

		//fmt.Printf("unmarshalled prom config: %+v\n", promCfg)
		cfg.PrometheusConfig.GlobalConfig = promCfg.GlobalConfig
		cfg.PrometheusConfig.ScrapeConfigs = promCfg.ScrapeConfigs
		for _, scfg := range cfg.PrometheusConfig.ScrapeConfigs {
			if scfg.HTTPClientConfig.TLSConfig.MaxVersion == 0 {
				scfg.HTTPClientConfig.TLSConfig.MaxVersion = tls.VersionTLS13
			}
			if scfg.HTTPClientConfig.TLSConfig.MinVersion == 0 {
				scfg.HTTPClientConfig.TLSConfig.MinVersion = tls.VersionTLS10
			}
		}
		// force update TLS min/max versions since they default to 0 as uint16 type and fails validations during marshalling
		if cfg.PrometheusConfig.TracingConfig.TLSConfig.MaxVersion == 0 {
			cfg.PrometheusConfig.TracingConfig.TLSConfig.MaxVersion = tls.VersionTLS13
		}
		if cfg.PrometheusConfig.TracingConfig.TLSConfig.MinVersion == 0 {
			cfg.PrometheusConfig.TracingConfig.TLSConfig.MinVersion = tls.VersionTLS10
		}
		//cfg.TargetAllocator = &promCfg.TargetAllocator
	}

	return cfg, nil
}
