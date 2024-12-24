// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emf_logs

import (
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awscloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/awsentity"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/tcplog"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/udplog"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

var (
	emfKey                         = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.Emf)
	serviceAddressEMFKey           = common.ConfigKey(emfKey, common.ServiceAddress)
	structuredLogKey               = common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.StructuredLog)
	serviceAddressStructuredLogKey = common.ConfigKey(structuredLogKey, common.ServiceAddress)
)

type translator struct {
	id component.ID
}

var _ common.Translator[*common.ComponentTranslators] = (*translator)(nil)

func NewTranslator() common.Translator[*common.ComponentTranslators] {
	return &translator{}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(component.DataTypeLogs, common.PipelineNameEmfLogs)
}

// Translate creates a pipeline for emf if emf logs are collected
// section is present.
func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	if conf == nil || !(conf.IsSet(emfKey) || conf.IsSet(structuredLogKey)) {
		// Using EMF since EMF is recommended with public document
		// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Embedded_Metric_Format_Generation_CloudWatch_Agent.html#CloudWatch_Embedded_Metric_Format_Generation_Install_Agent
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: emfKey}
	}
	translators := common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config](),
		Processors: common.NewTranslatorMap(batchprocessor.NewTranslatorWithNameAndSection(common.PipelineNameEmfLogs, common.LogsKey)), // EMF logs sit under metrics_collected in "logs"
		Exporters:  common.NewTranslatorMap(awscloudwatchlogs.NewTranslatorWithName(common.PipelineNameEmfLogs)),
		Extensions: common.NewTranslatorMap(agenthealth.NewTranslator(component.DataTypeLogs, []string{agenthealth.OperationPutLogEvents}),
			agenthealth.NewTranslatorWithStatusCode(component.MustNewType("statuscode"), nil, true),
		),
	}
	if !(context.CurrentContext().RunInContainer() && ecsutil.GetECSUtilSingleton().IsECS()) {
		translators.Processors.Set(awsentity.NewTranslatorWithEntityType(awsentity.Service, "emf", false))
	}
	if serviceAddress, ok := common.GetString(conf, serviceAddressEMFKey); ok {
		if strings.Contains(serviceAddress, common.Udp) {
			translators.Receivers.Set(udplog.NewTranslatorWithName(common.PipelineNameEmfLogs))
		} else {
			translators.Receivers.Set(tcplog.NewTranslatorWithName(common.PipelineNameEmfLogs))
		}
	} else if serviceAddress, ok = common.GetString(conf, serviceAddressStructuredLogKey); ok {
		if strings.Contains(serviceAddress, common.Udp) {
			translators.Receivers.Set(udplog.NewTranslatorWithName(common.PipelineNameEmfLogs))
		} else {
			translators.Receivers.Set(tcplog.NewTranslatorWithName(common.PipelineNameEmfLogs))
		}
	} else {
		translators.Receivers = common.NewTranslatorMap(
			tcplog.NewTranslatorWithName(common.PipelineNameEmfLogs),
			udplog.NewTranslatorWithName(common.PipelineNameEmfLogs),
		)

	}
	return &translators, nil
}
