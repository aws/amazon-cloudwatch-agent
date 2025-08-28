// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package applicationsignals

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsxray"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/debug"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/awsproxy"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/k8smetadata"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/awsapplicationsignals"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/awsentity"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricstransformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetection"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

type translator struct {
	signal pipeline.Signal
}

var _ common.PipelineTranslator = (*translator)(nil)

func NewTranslator(signal pipeline.Signal) common.PipelineTranslator {
	return &translator{
		signal,
	}
}

func (t *translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(t.signal, common.AppSignals)
}

func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	configKey, ok := common.AppSignalsConfigKeys[t.signal]
	if !ok {
		return nil, fmt.Errorf("no config key defined for signal: %s", t.signal)
	}
	if conf == nil || (!conf.IsSet(configKey[0]) && !conf.IsSet(configKey[1])) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configKey[0]}
	}

	translators := &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
	}

	// Add OTLP receivers
	otlps, err := otlp.ParseOtlpConfig(conf, common.AppSignals, "", t.signal, -1)
	if err == nil {
		for _, otlpConfig := range otlps {
			translators.Receivers.Set(otlp.NewTranslator(
				otlpConfig,
				otlp.WithSignal(t.signal),
				common.WithName(common.AppSignals)),
			)
		}
	}

	if t.signal == pipeline.SignalMetrics {
		translators.Processors.Set(metricstransformprocessor.NewTranslatorWithName(common.AppSignals))
	}

	translators.Processors.Set(resourcedetection.NewTranslator(resourcedetection.WithSignal(t.signal)))
	translators.Processors.Set(awsapplicationsignals.NewTranslator(awsapplicationsignals.WithSignal(t.signal)))

	// ECS is not in scope for entity association, so we only add the entity processor in non-ECS platforms
	isECS := ecsutil.GetECSUtilSingleton().IsECS()
	if t.signal == pipeline.SignalMetrics && !isECS {
		translators.Processors.Set(awsentity.NewTranslatorWithEntityType(awsentity.Service, common.AppSignals, false))
		if context.CurrentContext().KubernetesMode() != "" {
			translators.Extensions.Set(k8smetadata.NewTranslator())
		}
	}

	if enabled, _ := common.GetBool(conf, common.AgentDebugConfigKey); enabled {
		translators.Exporters.Set(debug.NewTranslator(common.WithName(common.AppSignals)))
	}

	if t.signal == pipeline.SignalTraces {
		translators.Exporters.Set(awsxray.NewTranslatorWithName(common.AppSignals))
		translators.Extensions.Set(awsproxy.NewTranslatorWithName(common.AppSignals))
		translators.Extensions.Set(agenthealth.NewTranslator(agenthealth.TracesName, []string{agenthealth.OperationPutTraceSegments}))
		translators.Extensions.Set(agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true))

	} else {
		translators.Exporters.Set(awsemf.NewTranslatorWithName(common.AppSignals))
		translators.Extensions.Set(agenthealth.NewTranslator(agenthealth.LogsName, []string{agenthealth.OperationPutLogEvents}))
		translators.Extensions.Set(agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true))
	}
	return translators, nil
}
