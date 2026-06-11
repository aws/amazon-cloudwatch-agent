// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributestocontextprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/forward"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/otlphttp"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/awscloudwatchlogsprovisioner"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/headerssetter"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/attributestocontext"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetection"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/transformprocessor"
)

type baseLogsTranslator struct{}

var _ common.PipelineTranslator = (*baseLogsTranslator)(nil)

func NewBaseLogsTranslator() common.PipelineTranslator {
	return &baseLogsTranslator{}
}

func (t *baseLogsTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalLogs, common.OpenTelemetryKey)
}

func (t *baseLogsTranslator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	otlpKey := common.ConfigKey(common.OpenTelemetryKey, common.CollectKey, common.OtlpKey)
	if conf == nil || (!conf.IsSet(common.OtelCollectLogsConfigKey) && !conf.IsSet(common.DatabaseInsightsConfigKey) && !conf.IsSet(otlpKey)) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: common.OtelCollectLogsConfigKey + " or " + common.DatabaseInsightsConfigKey + " or " + otlpKey}
	}

	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for %s logs pipeline", common.OpenTelemetryKey)
	}

	logsEndpoint := common.ServiceEndpoint("logs", region, "/v1/logs")

	// Extensions
	sigv4Ext := sigv4auth.NewTranslatorWithService("logs")
	provisionerExt := awscloudwatchlogsprovisioner.NewTranslator(sigv4Ext.ID())
	headersExt := headerssetter.NewTranslatorWithName("logs",
		headerssetter.WithAdditionalAuth(provisionerExt.ID()),
		headerssetter.WithHeaders([]headerssetter.HeaderMapping{
			{HeaderName: "x-aws-log-group", ContextKey: "aws.log.group.name"},
			{HeaderName: "x-aws-log-stream", ContextKey: "aws.log.stream.name"},
		}),
	)

	// Connector
	fwdConnector := forward.NewTranslator(common.OpenTelemetryKey)

	// Agent health
	agentHealthExt := agenthealth.NewTranslator(agenthealth.LogsName, []string{"*"}, agenthealth.WithAdditionalAuth(headersExt.ID()))

	// Processors
	attrCtx := attributestocontext.NewTranslator([]attributestocontextprocessor.ActionKeyValue{
		{Key: "aws.log.group.name", FromResourceAttribute: "aws.log.group.name"},
		{Key: "aws.log.stream.name", FromResourceAttribute: "aws.log.stream.name"},
	})
	logsCleanup := transformprocessor.NewTranslatorWithName("logs_cleanup",
		transformprocessor.WithLogStatements([]string{
			`delete_key(resource.attributes, "aws.log.group.name")`,
			`delete_key(resource.attributes, "aws.log.stream.name")`,
		}),
	)
	batch := batchprocessor.NewTranslator(
		common.WithName("logs"),
		batchprocessor.WithTimeout(1*time.Minute),
		batchprocessor.WithMetadataKeys([]string{"aws.log.group.name", "aws.log.stream.name"}),
	)

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
		Processors: common.NewTranslatorMap[component.Config, component.ID](resourcedetection.NewTranslator(), attrCtx, logsCleanup, batch),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](otlphttp.NewTranslatorWithName("logs", otlphttp.EndpointConfig{LogsEndpoint: logsEndpoint}, otlphttp.WithAuthenticator(agentHealthExt.ID()))),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](sigv4Ext, provisionerExt, headersExt, agentHealthExt),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
	}, nil
}
