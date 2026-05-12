// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package applicationsignals

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
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
		// For logs: also activate if metrics is enabled (auto-opt-in)
		if t.signal == pipeline.SignalLogs {
			metricsKeys := common.AppSignalsConfigKeys[pipeline.SignalMetrics]
			if !conf.IsSet(metricsKeys[0]) && !conf.IsSet(metricsKeys[1]) {
				return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configKey[0]}
			}
		} else {
			return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configKey[0]}
		}
	}

	// Logs uses a separate pipeline (otlphttp → CW OTLP endpoint) with no
	// shared processors or exporters with metrics/traces.
	if t.signal == pipeline.SignalLogs {
		return t.translateLogs(conf, configKey)
	}

	translators := &common.ComponentTranslators{
		Receivers:  otlp.NewTranslators(conf, common.AppSignals, t.signal.String()),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
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

func (t *translator) translateLogs(conf *confmap.Conf, configKeys []string) (*common.ComponentTranslators, error) {
	region := agent.Global_Config.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for %s logs pipeline", common.AppSignals)
	}

	// Use the logs config keys if explicitly set; otherwise fall back to metrics
	// config for the OTLP receiver (auto-opt-in: same receiver endpoints as metrics).
	receiverSignal := pipeline.SignalLogs.String()
	if !conf.IsSet(configKeys[0]) && !conf.IsSet(configKeys[1]) {
		receiverSignal = pipeline.SignalMetrics.String()
	}

	logGroupTemplate, logStreamTemplate := resolveLogConfig(conf, configKeys)

	sigv4AuthID := component.NewIDWithName(component.MustNewType("sigv4auth"), common.AppSignals)
	provisionerID := component.MustNewID("awscloudwatchlogsprovisioner")
	headersSetterID := component.NewIDWithName(component.MustNewType("headers_setter"), common.AppSignals)
	logsEndpoint := otlphttp.EndpointConfig{
		BaseEndpoint: fmt.Sprintf("https://logs.%s.amazonaws.com", region),
		LogsEndpoint: fmt.Sprintf("https://logs.%s.amazonaws.com/v1/logs", region),
	}

	var statements []string
	var attrActions []attributestocontext.ActionMapping
	var metadataKeys []string
	var headerMappings []headerssetter.HeaderMapping
	staticHeaders := map[string]string{}

	if hasPlaceholders(logGroupTemplate) {
		statements = append(statements, buildOTTLSetStatement(metadataKeyLogGroup, logGroupTemplate))
		attrActions = append(attrActions, attributestocontext.ActionMapping{Key: metadataKeyLogGroup, FromResourceAttribute: metadataKeyLogGroup})
		metadataKeys = append(metadataKeys, metadataKeyLogGroup)
		headerMappings = append(headerMappings, headerssetter.HeaderMapping{HeaderName: "x-aws-log-group", ContextKey: metadataKeyLogGroup})
	} else {
		staticHeaders["x-aws-log-group"] = templateToLiteral(logGroupTemplate)
	}

	if hasPlaceholders(logStreamTemplate) {
		statements = append(statements, buildOTTLSetStatement(metadataKeyLogStream, logStreamTemplate))
		attrActions = append(attrActions, attributestocontext.ActionMapping{Key: metadataKeyLogStream, FromResourceAttribute: metadataKeyLogStream})
		metadataKeys = append(metadataKeys, metadataKeyLogStream)
		headerMappings = append(headerMappings, headerssetter.HeaderMapping{HeaderName: "x-aws-log-stream", ContextKey: metadataKeyLogStream})
	} else {
		staticHeaders["x-aws-log-stream"] = templateToLiteral(logStreamTemplate)
	}

	dynamic := len(statements) > 0

	translators := &common.ComponentTranslators{
		Receivers:  otlp.NewTranslators(conf, common.AppSignals, receiverSignal),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
	}

	if dynamic {
		translators.Processors.Set(transformproc.NewTranslatorWithName(common.AppSignals, transformproc.WithLogStatements(statements)))
		translators.Processors.Set(attributestocontext.NewTranslator(attrActions))
		translators.Processors.Set(batchproc.NewTranslator(
			common.WithName(common.AppSignals),
			batchproc.WithMetadataKeys(metadataKeys),
		))
		translators.Exporters.Set(otlphttp.NewTranslatorWithName(common.AppSignals, logsEndpoint,
			otlphttp.WithAuthenticator(headersSetterID),
			otlphttp.WithHeaders(staticHeaders),
		))
		translators.Extensions.Set(headerssetter.NewTranslatorWithName(common.AppSignals,
			headerssetter.WithAdditionalAuth(provisionerID),
			headerssetter.WithHeaders(headerMappings),
		))
	} else {
		translators.Processors.Set(batchproc.NewTranslator(common.WithName(common.AppSignals)))
		translators.Exporters.Set(otlphttp.NewTranslatorWithName(common.AppSignals, logsEndpoint,
			otlphttp.WithAuthenticator(provisionerID),
			otlphttp.WithHeaders(staticHeaders),
		))
	}

	if enabled, _ := common.GetBool(conf, common.AgentDebugConfigKey); enabled {
		translators.Exporters.Set(debug.NewTranslator(common.WithName(common.AppSignals)))
	}

	translators.Extensions.Set(sigv4auth.NewTranslatorWithName(common.AppSignals, sigv4auth.WithService("logs")))
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
