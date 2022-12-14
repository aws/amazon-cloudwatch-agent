// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otel

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/telemetry"
	"go.uber.org/multierr"
	"go.uber.org/zap/zapcore"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	receiverAdapter "github.com/aws/private-amazon-cloudwatch-agent-staging/receiver/adapter"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/exporter/awscloudwatch"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/exporter/awsemf"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/extension"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/extension/ecsobserver"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/pipeline"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/pipeline/containerinsights"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/pipeline/host"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/pipeline/prometheus"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/processor"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/processor/cumulativetodeltaprocessor"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/processor/ec2taggerprocessor"
	metricstransformprocessor "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/processor/metricstransform"
	resourceprocessor "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/processor/resource"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/receiver/adapter"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/receiver/awscontainerinsight"
	prometheusreceiver "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/receiver/prometheus"
)

// Translator is used to create an OTEL config.
type Translator struct {
	receiverTranslators  common.TranslatorMap[component.Config]
	processorTranslators common.TranslatorMap[component.Config]
	exporterTranslators  common.TranslatorMap[component.Config]
	extensionTranslators common.TranslatorMap[component.Config]
}

// NewTranslator creates a new Translator.
func NewTranslator() *Translator {
	return &Translator{
		receiverTranslators: common.NewTranslatorMap(
			awscontainerinsight.NewTranslator(),
			prometheusreceiver.NewTranslator(),
		),
		processorTranslators: common.NewTranslatorMap(
			processor.NewDefaultTranslator(batchprocessor.NewFactory()),
			cumulativetodeltaprocessor.NewTranslator(),
			ec2taggerprocessor.NewTranslator(),
			metricstransformprocessor.NewTranslator(),
			resourceprocessor.NewTranslator(),
		),
		exporterTranslators: common.NewTranslatorMap(
			awscloudwatch.NewTranslator(),
			awsemf.NewTranslator(),
		),
		extensionTranslators: common.NewTranslatorMap(
			ecsobserver.NewTranslator(),
		),
	}
}

// Translate converts a JSON config into an OTEL config.
func (t *Translator) Translate(jsonConfig interface{}, os string) (*otelcol.Config, error) {
	m, ok := jsonConfig.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid json config")
	}
	conf := confmap.NewFromStringMap(m)

	found, err := adapter.FindReceiversInConfig(conf, os)
	if err != nil {
		return nil, fmt.Errorf("unable to find receivers in config: %w", err)
	}
	t.receiverTranslators.Merge(found)

	// split out delta receiver types
	receiverTypes := collections.Keys(found)
	var deltaMetricsReceivers []component.Type
	var hostReceiverTypes []component.Type
	for i := range receiverTypes {
		if receiverTypes[i] == receiverAdapter.TelegrafPrefix+common.DiskIOName || receiverTypes[i] == receiverAdapter.TelegrafPrefix+common.NetName {
			deltaMetricsReceivers = append(deltaMetricsReceivers, receiverTypes[i])
		} else {
			hostReceiverTypes = append(hostReceiverTypes, receiverTypes[i])
		}
	}

	pipelines, err := pipeline.NewTranslator(
		host.NewTranslator(hostReceiverTypes, common.HostPipelineName),
		host.NewTranslator(deltaMetricsReceivers, common.HostDeltaMetricsPipelineName),
		containerinsights.NewTranslator(),
		prometheus.NewTranslator(),
	).Translate(conf)
	if err != nil {
		return nil, fmt.Errorf("unable to translate pipelines: %w", err)
	}
	extensions, err := extension.NewTranslator(
		ecsobserver.NewTranslator(),
	).Translate(conf)
	cfg := &otelcol.Config{
		Receivers:  map[component.ID]component.Config{},
		Exporters:  map[component.ID]component.Config{},
		Processors: map[component.ID]component.Config{},
		Extensions: extensions,
		Service: service.ConfigService{
			Telemetry: telemetry.Config{
				Logs: telemetry.LogsConfig{
					Level:    zapcore.InfoLevel,
					Encoding: common.Json,
					Sampling: &telemetry.LogsSamplingConfig{
						Initial:    2,
						Thereafter: 500,
					}},
				Metrics: telemetry.MetricsConfig{Level: configtelemetry.LevelNone},
			},
			Pipelines:  pipelines,
			Extensions: collections.Keys(extensions),
		},
	}
	if err = t.buildComponents(cfg, conf); err != nil {
		return nil, fmt.Errorf("unable to build components in pipeline: %w", err)
	}
	if err = cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid otel config: %w", err)
	}
	return cfg, nil
}

// buildComponents uses the pipelines defined in the config to build the components.
func (t *Translator) buildComponents(cfg *otelcol.Config, conf *confmap.Conf) error {
	var errs error
	receivers := collections.NewSet[component.ID]()
	processors := collections.NewSet[component.ID]()
	exporters := collections.NewSet[component.ID]()
	for _, p := range cfg.Service.Pipelines {
		receivers.Add(p.Receivers...)
		processors.Add(p.Processors...)
		exporters.Add(p.Exporters...)
	}
	errs = multierr.Append(errs, buildComponents(conf, receivers, cfg.Receivers, t.receiverTranslators.Get))
	errs = multierr.Append(errs, buildComponents(conf, processors, cfg.Processors, t.processorTranslators.Get))
	errs = multierr.Append(errs, buildComponents(conf, exporters, cfg.Exporters, t.exporterTranslators.Get))
	return errs
}

// buildComponents attempts to translate a component for each ID in the set.
func buildComponents[C common.Identifiable](
	conf *confmap.Conf,
	ids collections.Set[component.ID],
	components map[component.ID]C,
	getTranslator func(component.Type) (common.Translator[C], bool),
) error {
	var errs error
	for id := range ids {
		translator, ok := getTranslator(id.Type())
		if !ok {
			errs = multierr.Append(errs, fmt.Errorf("missing translator for %v", id.Type()))
			continue
		}
		cfg, err := translator.Translate(conf)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		cfg.SetIDName(id.Name())
		components[cfg.ID()] = cfg
	}
	return errs
}
