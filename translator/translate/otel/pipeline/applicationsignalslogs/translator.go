// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// Package applicationsignalslogs translates logs.logs_collected.application_signals
// into an OTel logs pipeline that routes OTLP logs to CloudWatch via the CW OTLP
// endpoint.
//
// Two pipeline shapes depending on whether placeholders like {service.name} are
// used in log_group_name / log_stream_name:
//
// Dynamic (placeholders present):
//
//	receivers: [otlp]
//	processors: [transform, attributestocontext, batch(metadata_keys)]
//	exporters: [otlphttp]
//	extensions: [sigv4auth, awscloudwatchlogsprovisioner]
//
// Static (no placeholders):
//
//	receivers: [otlp]
//	processors: [batch]
//	exporters: [otlphttp(static headers)]
//	extensions: [sigv4auth, awscloudwatchlogsprovisioner]
package applicationsignalslogs

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/exporter/debug"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/otlp"
)

const (
	pipelineName = "application_signals_logs"

	// TODO: Update default log group prefix before PR is merged.
	defaultLogGroupPrefix = "/aws/telemetry/"
	defaultLogStreamName  = "default"
)

type translator struct{}

var _ common.PipelineTranslator = (*translator)(nil)

func NewTranslator() common.PipelineTranslator {
	return &translator{}
}

func (t *translator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalLogs, pipelineName)
}

func (t *translator) Translate(conf *confmap.Conf) (*common.ComponentTranslators, error) {
	configKeys := common.AppSignalsConfigKeys[pipeline.SignalLogs]
	if conf == nil || (!conf.IsSet(configKeys[0]) && !conf.IsSet(configKeys[1])) {
		return nil, &common.MissingKeyError{ID: t.ID(), JsonKey: configKeys[0]}
	}

	logGroupTemplate, logStreamTemplate := resolveLogConfig(conf, configKeys)
	dynamic := hasPlaceholders(logGroupTemplate) || hasPlaceholders(logStreamTemplate)

	translators := &common.ComponentTranslators{
		Receivers:  otlp.NewTranslators(conf, common.AppSignals, pipeline.SignalLogs.String()),
		Processors: common.NewTranslatorMap[component.Config, component.ID](),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
	}

	if dynamic {
		translators.Processors.Set(newTransformTranslator(logGroupTemplate, logStreamTemplate))
		translators.Processors.Set(newAttributesToContextTranslator())
		translators.Processors.Set(newBatchWithMetadataKeysTranslator())
		translators.Exporters.Set(newDynamicOTLPHTTPExporterTranslator())
		translators.Extensions.Set(newHeadersSetterTranslator())
	} else {
		translators.Processors.Set(newBatchTranslator())
		translators.Exporters.Set(newStaticOTLPHTTPExporterTranslator(map[string]string{
			"x-aws-log-group":  templateToLiteral(logGroupTemplate),
			"x-aws-log-stream": templateToLiteral(logStreamTemplate),
		}))
	}

	if enabled, _ := common.GetBool(conf, common.AgentDebugConfigKey); enabled {
		translators.Exporters.Set(debug.NewTranslator(common.WithName(pipelineName)))
	}

	// Extensions: sigv4auth + awscloudwatchlogsprovisioner (both paths need these)
	translators.Extensions.Set(newSigV4AuthTranslator())
	translators.Extensions.Set(newProvisionerTranslator())
	translators.Extensions.Set(agenthealth.NewTranslator(agenthealth.LogsName, []string{agenthealth.OperationPutLogEvents}))

	return translators, nil
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

// templateSegment represents either a literal string or an attribute reference
// in a log group/stream name template.
type templateSegment struct {
	literal   string
	attribute string // e.g. "service.name" — empty for literal segments
}

// resolveLogConfig reads log_group_name and log_stream_name from the config
// and parses them into template segments for OTTL Concat generation.
func resolveLogConfig(conf *confmap.Conf, configKeys []string) (logGroupTemplate, logStreamTemplate []templateSegment) {
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
		logGroupName = defaultLogGroupPrefix + "{service.name}"
	}

	return parseTemplate(logGroupName), parseTemplate(logStreamName)
}

// parseTemplate splits a template string like "/a/{service.name}/b/{attr}"
// into alternating literal and attribute segments.
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

// AutoEnableIfNeeded injects logs.logs_collected.application_signals with defaults
// when logs.metrics_collected.application_signals is configured but
// logs.logs_collected.application_signals is not.
// This auto-opt-in behavior ensures existing customers get the new OTLP logs
// pipeline without config changes on CWAgent upgrade.
func AutoEnableIfNeeded(conf map[string]interface{}) {
	logs, ok := conf["logs"].(map[string]interface{})
	if !ok {
		return
	}
	metricsCollected, ok := logs["metrics_collected"].(map[string]interface{})
	if !ok {
		return
	}
	_, hasAppSignals := metricsCollected["application_signals"]
	_, hasAppSignalsFallback := metricsCollected["app_signals"]
	if !hasAppSignals && !hasAppSignalsFallback {
		return
	}

	logsCollected, ok := logs["logs_collected"].(map[string]interface{})
	if !ok {
		logsCollected = map[string]interface{}{}
		logs["logs_collected"] = logsCollected
	}
	if _, exists := logsCollected["application_signals"]; exists {
		return
	}
	if _, exists := logsCollected["app_signals"]; exists {
		return
	}

	logsCollected["application_signals"] = map[string]interface{}{}
	fmt.Println("I! Auto-enabling logs.logs_collected.application_signals (triggered by logs.metrics_collected.application_signals)")
}
