// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package applicationsignals

import (
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/routing"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsemf"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/awsxray"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/debug"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/otlphttp"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/awscloudwatchlogsprovisioner"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/awsproxy"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/headerssetter"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/k8smetadata"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/sigv4auth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/attributestocontext"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/awsapplicationsignals"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/awsentity"
	batchproc "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/metricstransformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetection"
	transformproc "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/transformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

const (
	metricsComponentName   = "application_signals_metrics"
	metricsVariantRoute    = "application_signals_metrics_route"
	metricsVariantLogDest  = "application_signals_metrics_logs_destination"
	metricsVariantOtlpDest = "application_signals_metrics_otlp_destination"
)

const (
	logsComponentName  = "application_signals_logs"
	logsVariantRoute   = "application_signals_logs_route"
	logsVariantBatch   = "application_signals_logs_batch"
	logsVariantNoBatch = "application_signals_logs_nobatch"
)

const (
	defaultLogGroupName  = "/aws/service-events/{service.name}"
	defaultLogStreamName = "default"

	metadataKeyLogGroup  = "aws.cloudwatch.log_group.destination"
	metadataKeyLogStream = "aws.cloudwatch.log_stream.destination"
)

type templateSegment struct {
	literal   string
	attribute string
}

type translator struct {
	signal  pipeline.Signal
	variant string
}

var _ common.PipelineTranslator = (*translator)(nil)

func NewTranslator(signal pipeline.Signal, opts ...common.TranslatorOption) common.PipelineTranslator {
	t := &translator{signal: signal}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() pipeline.ID {
	if t.variant != "" {
		return pipeline.NewIDWithName(t.signal, t.variant)
	}
	return pipeline.NewIDWithName(t.signal, common.AppSignals)
}

func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	configKey, ok := common.AppSignalsConfigKeys[t.signal]
	if !ok {
		return nil, fmt.Errorf("no config key defined for signal: %s", t.signal)
	}
	if conf == nil {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configKey[0]}
	}
	if !conf.IsSet(configKey[0]) && (len(configKey) < 2 || !conf.IsSet(configKey[1])) {
		// For logs: also activate if metrics is enabled (auto-opt-in)
		if t.signal == pipeline.SignalLogs {
			metricsKey := common.AppSignalsConfigKeys[pipeline.SignalMetrics]
			if !conf.IsSet(metricsKey[0]) && (len(metricsKey) < 2 || !conf.IsSet(metricsKey[1])) {
				return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configKey[0]}
			}
		} else {
			return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configKey[0]}
		}
	}

	switch t.signal {
	case pipeline.SignalLogs:
		switch t.variant {
		case logsVariantRoute:
			return t.translateLogsReceiveToRoute(conf)
		case logsVariantBatch:
			return t.translateLogsRouteToOtlp(conf, true)
		case logsVariantNoBatch:
			return t.translateLogsRouteToOtlp(conf, false)
		}
	case pipeline.SignalMetrics:
		switch t.variant {
		case metricsVariantRoute:
			return t.translateMetricsReceiveToRoute(conf)
		case metricsVariantLogDest:
			return t.translateMetricsRouteToLogs(conf)
		case metricsVariantOtlpDest:
			return t.translateMetricsRouteToOtlp(conf)
		}
	}

	return t.translateTraces(conf)
}

func (t *translator) translateTraces(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	translators := &common.ComponentTranslators{
		Receivers:  otlp.NewTranslators(conf, common.AppSignals, t.signal.String()),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
	}

	translators.Processors.Set(resourcedetection.NewTranslator(resourcedetection.WithSignal(t.signal)))
	translators.Processors.Set(awsapplicationsignals.NewTranslator(awsapplicationsignals.WithSignal(t.signal)))

	if enabled, _ := common.GetBool(conf, common.AgentDebugConfigKey); enabled {
		translators.Exporters.Set(debug.NewTranslator(common.WithName(common.AppSignals)))
	}

	translators.Exporters.Set(awsxray.NewTranslatorWithName(common.AppSignals))
	translators.Extensions.Set(awsproxy.NewTranslatorWithName(common.AppSignals))
	translators.Extensions.Set(agenthealth.NewTranslator(agenthealth.TracesName, []string{agenthealth.OperationPutTraceSegments}))
	translators.Extensions.Set(agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true))

	return translators, nil
}

func newMetricsRoutingConnectorTranslator() common.ComponentTranslator {
	defaultPipelineID := pipeline.NewIDWithName(pipeline.SignalMetrics, metricsVariantLogDest)
	serviceEventsPipelineID := pipeline.NewIDWithName(pipeline.SignalMetrics, metricsVariantOtlpDest)

	return routing.NewTranslator(metricsComponentName,
		routing.WithErrorMode(ottl.IgnoreError),
		routing.WithDefaultPipelines(defaultPipelineID),
		routing.WithTable(routingconnector.RoutingTableItem{
			Context:   "datapoint",
			Condition: `attributes["Telemetry.Source"] == "ServiceEvents"`,
			Pipelines: []pipeline.ID{serviceEventsPipelineID},
		}),
	)
}

func (t *translator) translateMetricsReceiveToRoute(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	connectorTranslator := newMetricsRoutingConnectorTranslator()

	translators := &common.ComponentTranslators{
		Receivers:  otlp.NewTranslators(conf, common.AppSignals, pipeline.SignalMetrics.String()),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](),
	}

	translators.Exporters.Set(connectorTranslator)
	translators.Connectors.Set(connectorTranslator)

	return translators, nil
}

func (t *translator) translateMetricsRouteToLogs(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	connectorTranslator := newMetricsRoutingConnectorTranslator()

	translators := &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
	}

	translators.Receivers.Set(connectorTranslator)

	translators.Processors.Set(metricstransformprocessor.NewTranslatorWithName(common.AppSignals))
	translators.Processors.Set(resourcedetection.NewTranslator(resourcedetection.WithSignal(t.signal)))
	translators.Processors.Set(awsapplicationsignals.NewTranslator(awsapplicationsignals.WithSignal(t.signal)))

	isECS := ecsutil.GetECSUtilSingleton().IsECS()
	if !isECS {
		translators.Processors.Set(awsentity.NewTranslatorWithEntityType(awsentity.Service, common.AppSignals, false))
		if context.CurrentContext().KubernetesMode() != "" {
			translators.Extensions.Set(k8smetadata.NewTranslator())
		}
	}

	if enabled, _ := common.GetBool(conf, common.AgentDebugConfigKey); enabled {
		translators.Exporters.Set(debug.NewTranslator(common.WithName(common.AppSignals)))
	}

	translators.Exporters.Set(awsemf.NewTranslatorWithName(common.AppSignals))
	translators.Extensions.Set(agenthealth.NewTranslator(agenthealth.LogsName, []string{agenthealth.OperationPutLogEvents}))
	translators.Extensions.Set(agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true))

	return translators, nil
}

func (t *translator) translateMetricsRouteToOtlp(_ *confmap.Conf) (*common.ComponentTranslators, error) {
	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for %s OTLP metrics pipeline", common.AppSignals)
	}

	connectorTranslator := newMetricsRoutingConnectorTranslator()
	sigv4ID := component.NewIDWithName(component.MustNewType("sigv4auth"), metricsComponentName)
	metricsEndpoint := otlphttp.EndpointConfig{
		MetricsEndpoint: fmt.Sprintf("https://monitoring.%s.amazonaws.com/v1/metrics", region),
	}

	translators := &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
	}

	translators.Receivers.Set(connectorTranslator)

	translators.Processors.Set(batchproc.NewTranslator(common.WithName(metricsVariantOtlpDest), batchproc.WithTelemetrySection(common.LogsKey)))
	translators.Exporters.Set(otlphttp.NewTranslatorWithName(metricsVariantOtlpDest, metricsEndpoint,
		otlphttp.WithAuthenticator(sigv4ID),
	))
	translators.Extensions.Set(sigv4auth.NewTranslatorWithName(metricsComponentName, sigv4auth.WithService("monitoring")))
	translators.Extensions.Set(agenthealth.NewTranslator(agenthealth.LogsName, []string{agenthealth.OperationPutLogEvents}))

	return translators, nil
}

func newLogsRoutingConnectorTranslator() common.ComponentTranslator {
	batchPipelineID := pipeline.NewIDWithName(pipeline.SignalLogs, logsVariantBatch)
	noBatchPipelineID := pipeline.NewIDWithName(pipeline.SignalLogs, logsVariantNoBatch)

	return routing.NewTranslator(common.AppSignals+"_logs",
		routing.WithErrorMode(ottl.IgnoreError),
		routing.WithDefaultPipelines(batchPipelineID),
		routing.WithTable(routingconnector.RoutingTableItem{
			Context:   "log",
			Condition: `attributes["event.name"] == "aws.telemend.aggregate_profile"`,
			Pipelines: []pipeline.ID{noBatchPipelineID},
		}),
	)
}

func (t *translator) translateLogsReceiveToRoute(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for %s logs pipeline", common.AppSignals)
	}

	configKeys := common.AppSignalsConfigKeys[pipeline.SignalLogs]

	// Use logs receiver config if explicitly set; otherwise fall back to metrics
	// receiver config since both resolve to the same OTLP receiver (auto-opt-in
	// doesn't create logs.logs_collected.application_signals in the JSON config).
	receiverSignal := pipeline.SignalLogs.String()
	if !conf.IsSet(configKeys[0]) && !conf.IsSet(configKeys[1]) {
		receiverSignal = pipeline.SignalMetrics.String()
	}

	logGroupTemplate, logStreamTemplate := resolveLogConfig(conf, configKeys)
	dynamic := hasPlaceholders(logGroupTemplate) || hasPlaceholders(logStreamTemplate)

	var statements []string
	var attrActions []attributestocontext.ActionMapping

	if hasPlaceholders(logGroupTemplate) {
		statements = append(statements, buildOTTLSetStatement(metadataKeyLogGroup, logGroupTemplate))
		attrActions = append(attrActions, attributestocontext.ActionMapping{Key: metadataKeyLogGroup, FromResourceAttribute: metadataKeyLogGroup})
	}

	if hasPlaceholders(logStreamTemplate) {
		statements = append(statements, buildOTTLSetStatement(metadataKeyLogStream, logStreamTemplate))
		attrActions = append(attrActions, attributestocontext.ActionMapping{Key: metadataKeyLogStream, FromResourceAttribute: metadataKeyLogStream})
	}

	connectorTranslator := newLogsRoutingConnectorTranslator()

	translators := &common.ComponentTranslators{
		Receivers:  otlp.NewTranslators(conf, common.AppSignals, receiverSignal),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](),
	}

	if dynamic {
		translators.Processors.Set(transformproc.NewTranslatorWithName(logsComponentName, transformproc.WithLogStatements(statements)))
		translators.Processors.Set(attributestocontext.NewTranslator(attrActions))
	}

	translators.Exporters.Set(connectorTranslator)
	translators.Connectors.Set(connectorTranslator)

	return translators, nil
}

func (t *translator) translateLogsRouteToOtlp(conf *confmap.Conf, batch bool) (*common.ComponentTranslators, error) {
	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for %s logs pipeline", common.AppSignals)
	}

	configKeys := common.AppSignalsConfigKeys[pipeline.SignalLogs]
	logGroupTemplate, logStreamTemplate := resolveLogConfig(conf, configKeys)

	sigv4AuthID := component.NewIDWithName(component.MustNewType("sigv4auth"), logsComponentName)
	provisionerID := component.MustNewID("awscloudwatchlogsprovisioner")
	headersSetterID := component.NewIDWithName(component.MustNewType("headers_setter"), logsComponentName)
	logsEndpoint := otlphttp.EndpointConfig{
		BaseEndpoint: fmt.Sprintf("https://logs.%s.amazonaws.com", region),
		LogsEndpoint: fmt.Sprintf("https://logs.%s.amazonaws.com/v1/logs", region),
	}

	logGroupHasPlaceholders := hasPlaceholders(logGroupTemplate)
	logStreamHasPlaceholders := hasPlaceholders(logStreamTemplate)
	dynamic := logGroupHasPlaceholders || logStreamHasPlaceholders

	var metadataKeys []string
	var headerMappings []headerssetter.HeaderMapping

	if logGroupHasPlaceholders {
		metadataKeys = append(metadataKeys, metadataKeyLogGroup)
		headerMappings = append(headerMappings, headerssetter.HeaderMapping{HeaderName: "x-aws-log-group", ContextKey: metadataKeyLogGroup})
	} else {
		headerMappings = append(headerMappings, headerssetter.HeaderMapping{HeaderName: "x-aws-log-group", Value: templateToLiteral(logGroupTemplate)})
	}

	if logStreamHasPlaceholders {
		metadataKeys = append(metadataKeys, metadataKeyLogStream)
		headerMappings = append(headerMappings, headerssetter.HeaderMapping{HeaderName: "x-aws-log-stream", ContextKey: metadataKeyLogStream})
	} else {
		headerMappings = append(headerMappings, headerssetter.HeaderMapping{HeaderName: "x-aws-log-stream", Value: templateToLiteral(logStreamTemplate)})
	}

	connectorTranslator := newLogsRoutingConnectorTranslator()

	translators := &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
	}

	translators.Receivers.Set(connectorTranslator)

	if batch {
		if dynamic {
			translators.Processors.Set(batchproc.NewTranslator(
				common.WithName(logsComponentName),
				batchproc.WithMetadataKeys(metadataKeys),
				batchproc.WithTelemetrySection(common.LogsKey),
			))
		} else {
			translators.Processors.Set(batchproc.NewTranslator(
				common.WithName(logsComponentName),
				batchproc.WithTelemetrySection(common.LogsKey),
			))
		}
	}

	translators.Exporters.Set(otlphttp.NewTranslatorWithName(logsComponentName, logsEndpoint,
		otlphttp.WithAuthenticator(headersSetterID),
	))
	translators.Extensions.Set(headerssetter.NewTranslatorWithName(logsComponentName,
		headerssetter.WithAdditionalAuth(provisionerID),
		headerssetter.WithHeaders(headerMappings),
	))

	if enabled, _ := common.GetBool(conf, common.AgentDebugConfigKey); enabled {
		translators.Exporters.Set(debug.NewTranslator(common.WithName(logsComponentName)))
	}

	translators.Extensions.Set(sigv4auth.NewTranslatorWithName(logsComponentName, sigv4auth.WithService("logs")))
	translators.Extensions.Set(awscloudwatchlogsprovisioner.NewTranslator(sigv4AuthID))
	translators.Extensions.Set(agenthealth.NewTranslator(agenthealth.LogsName, []string{agenthealth.OperationPutLogEvents}))

	return translators, nil
}

func resolveLogConfig(conf *confmap.Conf, configKeys []string) ([]templateSegment, []templateSegment) {
	logGroupName := ""
	logStreamName := defaultLogStreamName

	for _, key := range configKeys {
		if v, ok := common.GetString(conf, common.ConfigKey(key, "log_group_name")); ok {
			logGroupName = v
		}
		if v, ok := common.GetString(conf, common.ConfigKey(key, "log_stream_name")); ok {
			logStreamName = v
		}
	}

	if logGroupName == "" {
		logGroupName = defaultLogGroupName
	}

	return parseTemplate(logGroupName), parseTemplate(logStreamName)
}

func parseTemplate(tmpl string) []templateSegment {
	var segments []templateSegment
	for len(tmpl) > 0 {
		openIdx := strings.Index(tmpl, "{")
		if openIdx < 0 {
			segments = append(segments, templateSegment{literal: tmpl})
			break
		}
		if openIdx > 0 {
			segments = append(segments, templateSegment{literal: tmpl[:openIdx]})
		}
		closeIdx := strings.Index(tmpl[openIdx:], "}")
		if closeIdx < 0 {
			segments = append(segments, templateSegment{literal: tmpl[openIdx:]})
			break
		}
		attrName := tmpl[openIdx+1 : openIdx+closeIdx]
		segments = append(segments, templateSegment{attribute: attrName})
		tmpl = tmpl[openIdx+closeIdx+1:]
	}
	return segments
}

func hasPlaceholders(segments []templateSegment) bool {
	for _, seg := range segments {
		if seg.attribute != "" {
			return true
		}
	}
	return false
}

func templateToLiteral(segments []templateSegment) string {
	var sb strings.Builder
	for _, seg := range segments {
		sb.WriteString(seg.literal)
	}
	return sb.String()
}

func buildOTTLSetStatement(metadataKey string, segments []templateSegment) string {
	whereGuard := fmt.Sprintf(` where resource.attributes["%s"] == nil`, metadataKey)

	hasAttributes := false
	for _, seg := range segments {
		if seg.attribute != "" {
			hasAttributes = true
			break
		}
	}

	if !hasAttributes {
		var literal string
		for _, seg := range segments {
			literal += seg.literal
		}
		return fmt.Sprintf(`set(resource.attributes["%s"], "%s")`, metadataKey, literal) + whereGuard
	}

	var parts []string
	for _, seg := range segments {
		if seg.attribute != "" {
			parts = append(parts, fmt.Sprintf(`resource.attributes["%s"]`, seg.attribute))
		} else {
			parts = append(parts, fmt.Sprintf(`"%s"`, seg.literal))
		}
	}
	return fmt.Sprintf(`set(resource.attributes["%s"], Concat([%s], ""))`, metadataKey, strings.Join(parts, ", ")) + whereGuard
}

// SetVariant implements common.TranslatorOption for setting the pipeline variant.
func SetVariant(variant string) common.TranslatorOption {
	return func(target any) {
		if t, ok := target.(*translator); ok {
			t.variant = variant
		}
	}
}
