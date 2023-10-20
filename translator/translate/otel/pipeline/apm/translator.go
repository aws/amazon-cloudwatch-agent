// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package apm

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsxray"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/awsproxy"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/awsapm"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetection"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

const (
	pipelineName = "apm"
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
	return component.NewIDWithName(t.dataType, pipelineName)
}

func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	configKey, ok := common.APMConfigKeys[t.dataType]
	if !ok {
		return nil, fmt.Errorf("no config key defined for data type: %s", t.dataType)
	}
	if conf == nil || !conf.IsSet(configKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configKey}
	}

	translators := &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap(otlp.NewTranslatorWithName(common.APM, otlp.WithDataType(t.dataType))),
		Processors: common.NewTranslatorMap(resourcedetection.NewTranslator(resourcedetection.WithDataType(t.dataType)), awsapm.NewTranslator(awsapm.WithDataType(t.dataType))),
		Exporters:  common.NewTranslatorMap[component.Config](),
		Extensions: common.NewTranslatorMap[component.Config](),
	}

	if t.dataType == component.DataTypeTraces {
		translators.Exporters.Set(awsxray.NewTranslatorWithName(common.APM))
		translators.Extensions.Set(awsproxy.NewTranslatorWithName(common.APM))
	} else {
		translators.Exporters.Set(awsemf.NewTranslatorWithName(common.APM))
	}
	return translators, nil
}
