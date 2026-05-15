// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslog

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/processor/syslogrouterprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	syslogroutertranslator "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/syslogrouter"
	syslogreceiver "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/syslog"
)

var syslogKey = common.ConfigKey(common.LogsKey, common.LogsCollectedKey, "syslog")

var _ common.PipelineTranslator = (*translator)(nil)

type translator struct {
	pipelineID           pipeline.ID
	receiverTranslator   common.ComponentTranslator
	routerTranslator     common.ComponentTranslator
	batchTranslator      common.ComponentTranslator
	exporterTranslator   common.ComponentTranslator
	extensionTranslators []common.ComponentTranslator
}

func (t *translator) ID() pipeline.ID {
	return t.pipelineID
}

func (t *translator) Translate(_ *confmap.Conf) (*common.ComponentTranslators, error) {
	translators := &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap(t.receiverTranslator),
		Processors: common.NewTranslatorMap(t.routerTranslator, t.batchTranslator),
		Exporters:  common.NewTranslatorMap(t.exporterTranslator),
		Extensions: common.NewTranslatorMap(t.extensionTranslators...),
	}
	return translators, nil
}

func NewTranslators(conf *confmap.Conf) common.PipelineTranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()
	if conf == nil || !conf.IsSet(syslogKey) {
		return translators
	}

	var listeners []any
	switch v := conf.Get(syslogKey).(type) {
	case map[string]any:
		listeners = []any{v}
	case []any:
		listeners = v
	default:
		return translators
	}

	for l, entry := range listeners {
		listenerMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		listenAddress, _ := listenerMap["listen_address"].(string)
		receiverName := deriveReceiverName(listenAddress)
		protocol, _ := listenerMap["protocol"].(string)
		tlsConfig := toTLSConfig(listenerMap)
		receiverTrans := syslogreceiver.NewTranslator(receiverName, listenAddress, protocol, tlsConfig)

		defaultLogGroupName, _ := listenerMap["log_group_name"].(string)
		defaultLogStreamName, _ := listenerMap["log_stream_name"].(string)
		if defaultLogStreamName == "" {
			defaultLogStreamName = "{hostname}"
		}
		defaultRetention := toInt64(listenerMap, "retention_in_days")

		var rules []map[string]any
		if rawRules, ok := listenerMap["routing"].([]any); ok {
			for _, r := range rawRules {
				if rm, ok := r.(map[string]any); ok {
					rules = append(rules, rm)
				}
			}
		}

		listenerFilters := toFilters(listenerMap)

		var allMatchRules []syslogrouterprocessor.MatchRule
		for _, rule := range rules {
			allMatchRules = append(allMatchRules, toMatchRule(rule))
		}

		for r, rule := range rules {
			pipelineName := fmt.Sprintf("syslog_%d_rule_%d", l, r)
			matchRule := toMatchRule(rule)
			routerCfg := syslogrouterprocessor.Config{
				Rule:            matchRule,
				PriorRules:      allMatchRules[:r],
				ListenerFilters: listenerFilters,
				RuleFilters:     toFilters(rule),
			}
			ruleLogGroup, _ := rule["log_group_name"].(string)
			ruleLogStream, _ := rule["log_stream_name"].(string)
			if ruleLogStream == "" {
				ruleLogStream = defaultLogStreamName
			}
			ruleRetention := toInt64(rule, "retention_in_days")
			if ruleRetention == 0 {
				ruleRetention = defaultRetention
			}
			translators.Set(&translator{
				pipelineID:           pipeline.NewIDWithName(pipeline.SignalLogs, pipelineName),
				receiverTranslator:   receiverTrans,
				routerTranslator:     syslogroutertranslator.NewTranslator(pipelineName, &routerCfg),
				batchTranslator:      batchprocessor.NewTranslatorWithNameAndSection(pipelineName, common.LogsKey),
				exporterTranslator:   newCWLExporterTranslator(pipelineName, ruleLogGroup, ruleLogStream, ruleRetention),
				extensionTranslators: newExtensionTranslators(),
			})
		}

		defaultPipelineName := fmt.Sprintf("syslog_%d_default", l)
		defaultRouterCfg := syslogrouterprocessor.Config{
			IsDefault:       true,
			AllRules:        allMatchRules,
			ListenerFilters: listenerFilters,
		}
		translators.Set(&translator{
			pipelineID:           pipeline.NewIDWithName(pipeline.SignalLogs, defaultPipelineName),
			receiverTranslator:   receiverTrans,
			routerTranslator:     syslogroutertranslator.NewTranslator(defaultPipelineName, &defaultRouterCfg),
			batchTranslator:      batchprocessor.NewTranslatorWithNameAndSection(defaultPipelineName, common.LogsKey),
			exporterTranslator:   newCWLExporterTranslator(defaultPipelineName, defaultLogGroupName, defaultLogStreamName, defaultRetention),
			extensionTranslators: newExtensionTranslators(),
		})
	}

	return translators
}

func newExtensionTranslators() []common.ComponentTranslator {
	return []common.ComponentTranslator{
		agenthealth.NewTranslator(agenthealth.LogsName, []string{agenthealth.OperationPutLogEvents}),
		agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true),
	}
}

func deriveReceiverName(listenAddress string) string {
	name := strings.ReplaceAll(listenAddress, "://", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, ":", "_")
	return name
}

func toMatchRule(rule map[string]any) syslogrouterprocessor.MatchRule {
	var mr syslogrouterprocessor.MatchRule
	matchMap, ok := rule["match"].(map[string]any)
	if !ok {
		return mr
	}
	if v, ok := matchMap["hostname"].(string); ok {
		mr.Hostname = v
	}
	if v, ok := matchMap["app_name"].(string); ok {
		mr.AppName = v
	}
	if v, ok := matchMap["facility"]; ok {
		var fac int
		switch f := v.(type) {
		case float64:
			fac = int(f)
		case int:
			fac = f
		default:
			return mr
		}
		mr.Facility = &fac
	}
	return mr
}

func toFilters(m map[string]any) []syslogrouterprocessor.Filter {
	rawFilters, ok := m["filters"].([]any)
	if !ok {
		return nil
	}
	var filters []syslogrouterprocessor.Filter
	for _, rf := range rawFilters {
		fm, ok := rf.(map[string]any)
		if !ok {
			continue
		}
		f := syslogrouterprocessor.Filter{}
		if v, ok := fm["type"].(string); ok {
			f.Type = v
		}
		if v, ok := fm["expression"].(string); ok {
			f.Expression = v
		}
		filters = append(filters, f)
	}
	return filters
}

func toInt64(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	}
	return 0
}

func toTLSConfig(m map[string]any) map[string]any {
	tlsMap, ok := m["tls"].(map[string]any)
	if !ok {
		return nil
	}
	cfg := make(map[string]any)
	if v, ok := tlsMap["cert_file"].(string); ok {
		cfg["cert_file"] = v
	}
	if v, ok := tlsMap["key_file"].(string); ok {
		cfg["key_file"] = v
	}
	if v, ok := tlsMap["ca_file"].(string); ok {
		cfg["ca_file"] = v
	}
	if v, ok := tlsMap["client_ca_file"].(string); ok {
		cfg["client_ca_file"] = v
	}
	if v, ok := tlsMap["min_version"].(string); ok {
		cfg["min_version"] = v
	}
	if len(cfg) == 0 {
		return nil
	}
	return cfg
}
