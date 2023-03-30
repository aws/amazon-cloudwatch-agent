// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package xray

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor/batchprocessor"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	awsxrayexporter "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/exporter/awsxray"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/processor"
	awsxrayreceiver "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/receiver/awsxray"
)

const (
	pipelineName = "xray"
)

var (
	baseKey = common.ConfigKey(common.TracesKey, common.TracesCollectedKey, common.XrayKey)
)

type translator struct {
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator() common.Translator[*common.ComponentTranslators] {
	return &translator{}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(component.DataTypeTraces, pipelineName)
}

func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !conf.IsSet(baseKey) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: baseKey}
	}
	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap(awsxrayreceiver.NewTranslator()),
		Processors: common.NewTranslatorMap(processor.NewDefaultTranslatorWithName(pipelineName, batchprocessor.NewFactory())),
		Exporters:  common.NewTranslatorMap(awsxrayexporter.NewTranslator()),
	}, nil
}
