// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"fmt"
	"os"
	"path/filepath"

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
	pipelineName = "otel_prometheus"
)

var prometheusKey = common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.PrometheusKey)
var configPathKey = common.ConfigKey(prometheusKey, "config_path")

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
	processors.Set(transformprocessor.NewTranslatorWithName("prometheus_scope",
		transformprocessor.WithErrorMode("ignore"),
		transformprocessor.WithMetricScopeStatements(common.ScopeStatementsForSolution("otel-prometheus")),
	))
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
	return component.NewIDWithName(prometheusreceiver.NewFactory().Type(), "opentelemetry")
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
	configPath = filepath.Clean(configPath)

	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read prometheus config from path %s: %w", configPath, err)
	}

	// Prevent OTel expandconverter from misinterpreting Prometheus regex backreferences.
	escaped := common.EscapeDollarDigit(string(content))
	var stringMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(escaped), &stringMap); err != nil {
		return nil, fmt.Errorf("unable to parse prometheus config from %s: %w", configPath, err)
	}

	componentParser := confmap.NewFromStringMap(stringMap)
	var promCfg prometheusreceiver.PromConfig
	if err := componentParser.Unmarshal(&promCfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal prometheus config from %s: %w", configPath, err)
	}
	cfg.PrometheusConfig.GlobalConfig = promCfg.GlobalConfig
	cfg.PrometheusConfig.ScrapeConfigs = promCfg.ScrapeConfigs
	cfg.PrometheusConfig.TracingConfig = promCfg.TracingConfig

	return cfg, nil
}
