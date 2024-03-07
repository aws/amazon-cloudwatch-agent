// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nginx

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awscloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/nginx"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

const (
	pipelineName = "nginx"
)

var (
	nginxKey = common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, "nginx")
)

type translator struct {
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator() common.Translator[*common.ComponentTranslators] {
	return &translator{}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(component.DataTypeMetrics, pipelineName)
}

func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !(conf.IsSet(nginxKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: nginxKey}
	}
	translators := &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap(nginx.NewTranslator()),
		Processors: common.NewTranslatorMap[component.Config](),
		Exporters:  common.NewTranslatorMap(awscloudwatch.NewTranslator()),
		Extensions: common.NewTranslatorMap[component.Config](),
	}
	return translators, nil
}
