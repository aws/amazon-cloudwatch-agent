// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otel

import (
	"errors"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/telemetry"
	"go.uber.org/multierr"
	"go.uber.org/zap/zapcore"

	receiverAdapter "github.com/aws/amazon-cloudwatch-agent/receiver/adapter"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/applicationsignals"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/containerinsights"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/emf_logs"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/host"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/prometheus"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/pipeline/xray"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/adapter"
)

var registry = common.NewTranslatorMap[*common.ComponentTranslators]()

func RegisterPipeline(translators ...pipeline.Translator) {
	for _, translator := range translators {
		registry.Set(translator)
	}
}

// Translate converts a JSON config into an OTEL config.
func Translate(jsonConfig interface{}, os string) (*otelcol.Config, error) {
	m, ok := jsonConfig.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid json config")
	}
	conf := confmap.NewFromStringMap(m)

	if conf.IsSet("csm") {
		log.Printf("W! CSM has already been deprecated")
	}

	adapterReceivers, err := adapter.FindReceiversInConfig(conf, os)
	if err != nil {
		return nil, fmt.Errorf("unable to find receivers in config: %w", err)
	}

	// split out delta receiver types
	deltaMetricsReceivers := common.NewTranslatorMap[component.Config]()
	hostReceivers := common.NewTranslatorMap[component.Config]()
	adapterReceivers.Range(func(translator common.Translator[component.Config]) {
		if translator.ID().Type() == receiverAdapter.Type(common.DiskIOKey) || translator.ID().Type() == receiverAdapter.Type(common.NetKey) {
			deltaMetricsReceivers.Set(translator)
		} else {
			hostReceivers.Set(translator)
		}
	})
	translators := common.NewTranslatorMap(
		applicationsignals.NewTranslator(component.DataTypeTraces),
		applicationsignals.NewTranslator(component.DataTypeMetrics),
		host.NewTranslator(common.PipelineNameHost, hostReceivers),
		host.NewTranslator(common.PipelineNameHostDeltaMetrics, deltaMetricsReceivers),
		containerinsights.NewTranslator(),
		prometheus.NewTranslator(),
		emf_logs.NewTranslator(),
		xray.NewTranslator(),
	)
	translators.Merge(registry)
	pipelines, err := pipeline.NewTranslator(translators).Translate(conf)
	if err != nil {
		return nil, err
	}
	cfg := &otelcol.Config{
		Receivers:  map[component.ID]component.Config{},
		Exporters:  map[component.ID]component.Config{},
		Processors: map[component.ID]component.Config{},
		Extensions: map[component.ID]component.Config{},
		Service: service.Config{
			Telemetry: telemetry.Config{
				Logs:    getLoggingConfig(conf),
				Metrics: telemetry.MetricsConfig{Level: configtelemetry.LevelNone},
			},
			Pipelines:  pipelines.Pipelines,
			Extensions: pipelines.Translators.Extensions.Keys(),
		},
	}
	if err = build(conf, cfg, pipelines.Translators); err != nil {
		return nil, fmt.Errorf("unable to build components in pipeline: %w", err)
	}
	if err = cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid otel config: %w", err)
	}
	return cfg, nil
}

// parseAgentLogLevel returns the logging level from the JSON config, or the
// default value.
func parseAgentLogLevel(conf *confmap.Conf) zapcore.Level {
	// "quiet" takes precedence over "debug" in Telegraf.
	v, _ := common.GetBool(conf, common.ConfigKey("agent", "quiet"))
	if v {
		return zapcore.ErrorLevel
	}
	v, _ = common.GetBool(conf, common.ConfigKey("agent", "debug"))
	if v {
		return zapcore.DebugLevel
	}
	return zapcore.InfoLevel
}

// getLoggingConfig uses the given JSON config to determine the correct
// logging configuration that should go in the YAML.
func getLoggingConfig(conf *confmap.Conf) telemetry.LogsConfig {
	var outputPaths []string
	filename := context.CurrentContext().GetAgentLogFile()
	// A slice with an empty string causes OTEL issues, so avoid it.
	if filename != "" {
		outputPaths = []string{filename}
	}
	logLevel := parseAgentLogLevel(conf)
	return telemetry.LogsConfig{
		OutputPaths: outputPaths,
		Level:       logLevel,
		Encoding:    common.Console,
		// enabled by default with 10 second tick
		Sampling: &telemetry.LogsSamplingConfig{
			Enabled:    true,
			Initial:    2,
			Thereafter: 500,
			Tick:       10 * time.Second,
		},
	}
}

// build uses the pipelines and extensions defined in the config to build the components.
func build(conf *confmap.Conf, cfg *otelcol.Config, translators common.ComponentTranslators) error {
	errs := buildComponents(conf, cfg.Service.Extensions, cfg.Extensions, translators.Extensions.Get)
	for _, p := range cfg.Service.Pipelines {
		errs = multierr.Append(errs, buildComponents(conf, p.Receivers, cfg.Receivers, translators.Receivers.Get))
		errs = multierr.Append(errs, buildComponents(conf, p.Processors, cfg.Processors, translators.Processors.Get))
		errs = multierr.Append(errs, buildComponents(conf, p.Exporters, cfg.Exporters, translators.Exporters.Get))
	}
	return errs
}

// buildComponents attempts to translate a component for each ID in the set.
func buildComponents[C component.Config](
	conf *confmap.Conf,
	ids []component.ID,
	components map[component.ID]C,
	getTranslator func(component.ID) (common.Translator[C], bool),
) error {
	var errs error
	for _, id := range ids {
		translator, ok := getTranslator(id)
		if !ok {
			errs = multierr.Append(errs, fmt.Errorf("missing translator for %v", id.Type()))
			continue
		}
		cfg, err := translator.Translate(conf)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		components[id] = cfg
	}
	return errs
}
