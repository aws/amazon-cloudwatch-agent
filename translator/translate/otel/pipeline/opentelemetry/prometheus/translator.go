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
	"go.opentelemetry.io/collector/pipeline"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/forward"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/transformprocessor"
)

const (
	pipelineName           = "prometheus"
	otelConfigParsingError = "has invalid keys: global"
	defaultTLSCaPath       = "/etc/amazon-cloudwatch-observability-agent-cert/tls-ca.crt"
	defaultTLSCertPath     = "/etc/amazon-cloudwatch-observability-agent-ta-client-cert/client.crt"
	defaultTLSKeyPath      = "/etc/amazon-cloudwatch-observability-agent-ta-client-cert/client.key"
)

var prometheusKey = common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.PrometheusKey)
var configPathKey = common.ConfigKey(prometheusKey, "config_path")
var clusterNameKey = common.ConfigKey(prometheusKey, "cluster_name")

type translator struct{}

var _ common.PipelineTranslator = (*translator)(nil)

func NewTranslator() common.PipelineTranslator {
	return &translator{}
}

func (t *translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalMetrics, pipelineName)
}

func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(prometheusKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: prometheusKey}
	}

	fwdConnector := forward.NewTranslator(common.OpenTelemetryKey)
	receiver := &prometheusReceiverTranslator{}

	processors := common.NewTranslatorMap[component.Config, component.ID]()
	if clusterName, ok := common.GetString(conf, clusterNameKey); ok && clusterName != "" {
		// Validate cluster_name to prevent OTTL injection (must be alphanumeric, hyphens, dots, underscores)
		for _, c := range clusterName {
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '.' && c != '_' {
				return nil, fmt.Errorf("cluster_name contains invalid character %q", c)
			}
		}
		processors.Set(transformprocessor.NewTranslatorWithName("set_cluster_name",
			transformprocessor.WithMetricStatements([]string{
				fmt.Sprintf(`set(resource.attributes["k8s.cluster.name"], "%s")`, clusterName),
			}),
		))
	}

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](receiver),
		Processors: processors,
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
	}, nil
}

// prometheusReceiverTranslator reads the prometheus config from config_path.
type prometheusReceiverTranslator struct{}

var _ common.ComponentTranslator = (*prometheusReceiverTranslator)(nil)

func (t *prometheusReceiverTranslator) ID() component.ID {
	return component.NewIDWithName(prometheusreceiver.NewFactory().Type(), "")
}

func (t *prometheusReceiverTranslator) Translate(conf *confmap.Conf) (component.Config, error) {
	factory := prometheusreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*prometheusreceiver.Config)

	if !conf.IsSet(configPathKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configPathKey}
	}

	configPath, _ := common.GetString(conf, configPathKey)
	if configPath == "" {
		return nil, fmt.Errorf("config_path must not be empty for opentelemetry prometheus receiver")
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read prometheus config from path %s: %w", configPath, err)
	}

	var stringMap map[string]interface{}
	if err := yaml.Unmarshal(content, &stringMap); err != nil {
		return nil, fmt.Errorf("unable to parse prometheus config from %s: %w", configPath, err)
	}

	componentParser := confmap.NewFromStringMap(stringMap)
	// NOTE: Prometheus relabel configs use $1, $2 for capture group references.
	// The OTel confmap expandconverter interprets these as environment variable
	// references (os.Expand), which can cause failures or empty replacements.
	// This is a known limitation when prometheus configs with relabel_configs
	// are loaded through the OTel config resolver pipeline.
	if err := componentParser.Unmarshal(&cfg); err != nil {
		// Config is in plain prometheus format, not OTel wrapper
		if !strings.Contains(err.Error(), otelConfigParsingError) {
			return nil, fmt.Errorf("unable to unmarshal prometheus config from %s: %w", configPath, err)
		}

		var promCfg prometheusreceiver.PromConfig
		if err := componentParser.Unmarshal(&promCfg); err != nil {
			return nil, fmt.Errorf("unable to unmarshal plain prometheus config from %s: %w", configPath, err)
		}
		cfg.PrometheusConfig.GlobalConfig = promCfg.GlobalConfig
		cfg.PrometheusConfig.ScrapeConfigs = promCfg.ScrapeConfigs
		cfg.PrometheusConfig.TracingConfig = promCfg.TracingConfig
	} else {
		// OTel format — check if target allocator is configured
		if cfg.TargetAllocator != nil && len(cfg.TargetAllocator.CollectorID) > 0 {
			cfg.TargetAllocator.TLSSetting.CAFile = defaultTLSCaPath
			cfg.TargetAllocator.TLSSetting.CertFile = defaultTLSCertPath
			cfg.TargetAllocator.TLSSetting.KeyFile = defaultTLSKeyPath
			cfg.TargetAllocator.TLSSetting.ReloadInterval = 10 * time.Second
		}
	}

	return cfg, nil
}
