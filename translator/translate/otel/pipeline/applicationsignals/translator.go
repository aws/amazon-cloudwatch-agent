// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package applicationsignals

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsxray"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/debug"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/awsproxy"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/awsapplicationsignals"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/awsentity"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricstransformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetection"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

type translator struct {
	dataType component.DataType
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator(dataType component.DataType) common.Translator[*common.ComponentTranslators] {
	return &translator{
		dataType,
	}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.dataType, common.AppSignals)
}

func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	configKey, ok := common.AppSignalsConfigKeys[t.dataType]
	if !ok {
		return nil, fmt.Errorf("no config key defined for data type: %s", t.dataType)
	}
	if conf == nil || (!conf.IsSet(configKey[0]) && !conf.IsSet(configKey[1])) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configKey[0]}
	}

	translators := &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap(otlp.NewTranslator(common.WithName(common.AppSignals), otlp.WithDataType(t.dataType))),
		Processors: common.NewTranslatorMap[component.Config](),
		Exporters:  common.NewTranslatorMap[component.Config](),
		Extensions: common.NewTranslatorMap[component.Config](),
	}

	if t.dataType == component.DataTypeMetrics {
		translators.Processors.Set(metricstransformprocessor.NewTranslatorWithName(common.AppSignals))
	}

	translators.Processors.Set(resourcedetection.NewTranslator(resourcedetection.WithDataType(t.dataType)))
	translators.Processors.Set(awsapplicationsignals.NewTranslator(awsapplicationsignals.WithDataType(t.dataType)))

	// ECS is not in scope for entity association, so we only add the entity processor in non-ECS platforms
	isECS := ecsutil.GetECSUtilSingleton().IsECS()
	if t.dataType == component.DataTypeMetrics && !isECS {
		translators.Processors.Set(awsentity.NewTranslatorWithEntityType(awsentity.Service, common.AppSignals, false))
	}

	if enabled, _ := common.GetBool(conf, common.AgentDebugConfigKey); enabled {
		translators.Exporters.Set(debug.NewTranslator(common.WithName(common.AppSignals)))
	}

	if t.dataType == component.DataTypeTraces {
		translators.Exporters.Set(awsxray.NewTranslatorWithName(common.AppSignals))
		translators.Extensions.Set(awsproxy.NewTranslatorWithName(common.AppSignals))
		translators.Extensions.Set(agenthealth.NewTranslator(component.DataTypeTraces, []string{agenthealth.OperationPutTraceSegments}))
		translators.Extensions.Set(agenthealth.NewTranslatorWithStatusCode(component.MustNewType("statuscode"), nil, true))

	} else {
		translators.Exporters.Set(awsemf.NewTranslatorWithName(common.AppSignals))
		translators.Extensions.Set(agenthealth.NewTranslator(component.DataTypeLogs, []string{agenthealth.OperationPutLogEvents}))
		translators.Extensions.Set(agenthealth.NewTranslatorWithStatusCode(component.MustNewType("statuscode"), nil, true))
	}
	return translators, nil
}
