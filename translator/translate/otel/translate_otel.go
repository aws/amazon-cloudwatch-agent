// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otel

import (
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/telemetry"
	"go.uber.org/multierr"
	"go.uber.org/zap/zapcore"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/exporter/awscloudwatch"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/exporter/awsemf"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/pipeline"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/pipeline/containerinsights"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/pipeline/host"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/processor"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/receiver/adapter"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/receiver/awscontainerinsight"
)

// Translator is used to create an OTEL config.
type Translator struct {
	receiverTranslators  common.TranslatorMap[config.Receiver]
	processorTranslators common.TranslatorMap[config.Processor]
	exporterTranslators  common.TranslatorMap[config.Exporter]
}

// NewTranslator creates a new Translator.
func NewTranslator() *Translator {
	return &Translator{
		receiverTranslators: common.NewTranslatorMap(
			awscontainerinsight.NewTranslator(),
		),
		processorTranslators: common.NewTranslatorMap(
			processor.NewDefaultTranslator(batchprocessor.NewFactory()),
			processor.NewDefaultTranslator(cumulativetodeltaprocessor.NewFactory()),
		),
		exporterTranslators: common.NewTranslatorMap(
			awscloudwatch.NewTranslator(),
			awsemf.NewTranslator(),
		),
	}
}

// Translate converts a JSON config into an OTEL config.
func (t *Translator) Translate(jsonConfig interface{}, os string) (*service.Config, error) {
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

	pipelines, err := pipeline.NewTranslator(
		host.NewTranslator(collections.Keys(found)),
		containerinsights.NewTranslator(),
	).Translate(conf)
	if err != nil {
		return nil, fmt.Errorf("unable to translate pipelines: %w", err)
	}
	cfg := &service.Config{
		Receivers:  map[config.ComponentID]config.Receiver{},
		Exporters:  map[config.ComponentID]config.Exporter{},
		Processors: map[config.ComponentID]config.Processor{},
		Service: service.ConfigService{
			Telemetry: telemetry.Config{
				Logs:    telemetry.LogsConfig{Level: zapcore.InfoLevel},
				Metrics: telemetry.MetricsConfig{Level: configtelemetry.LevelNone},
			},
			Pipelines: pipelines,
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
func (t *Translator) buildComponents(cfg *service.Config, conf *confmap.Conf) error {
	var errs error
	receivers := collections.NewSet[config.ComponentID]()
	processors := collections.NewSet[config.ComponentID]()
	exporters := collections.NewSet[config.ComponentID]()
	for _, p := range cfg.Pipelines {
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
	ids collections.Set[config.ComponentID],
	components map[config.ComponentID]C,
	getTranslator func(config.Type) (common.Translator[C], bool),
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
