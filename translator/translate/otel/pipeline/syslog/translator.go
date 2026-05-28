// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package syslog

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/batchprocessor"
	syslogreceiver "github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/syslog"
)

// simplePipelineTranslator produces a direct pipeline without routing: receivers → processors → exporter
type simplePipelineTranslator struct {
	pipelineID pipeline.ID
	receivers  []common.ComponentTranslator
	processors []common.ComponentTranslator
	exporter   common.ComponentTranslator
	extensions []common.ComponentTranslator
}

var _ common.PipelineTranslator = (*simplePipelineTranslator)(nil)

func (t *simplePipelineTranslator) ID() pipeline.ID { return t.pipelineID }

func (t *simplePipelineTranslator) Translate(_ *confmap.Conf) (*common.ComponentTranslators, error) {
	receivers := common.NewTranslatorMap[component.Config, component.ID]()
	for _, r := range t.receivers {
		receivers.Set(r)
	}
	processors := common.NewTranslatorMap[component.Config, component.ID]()
	for _, p := range t.processors {
		processors.Set(p)
	}
	exporters := common.NewTranslatorMap[component.Config, component.ID]()
	exporters.Set(t.exporter)
	extensions := common.NewTranslatorMap[component.Config, component.ID]()
	for _, e := range t.extensions {
		extensions.Set(e)
	}
	return &common.ComponentTranslators{
		Receivers:  receivers,
		Processors: processors,
		Exporters:  exporters,
		Extensions: extensions,
	}, nil
}

// inputPipelineTranslator produces the input pipeline: receivers → [filter] → routing connector
type inputPipelineTranslator struct {
	pipelineID          pipeline.ID
	receivers           []common.ComponentTranslator
	processors          []common.ComponentTranslator
	connectorExporterID common.ComponentTranslator // routing connector appears as exporter in this pipeline
	extensions          []common.ComponentTranslator
	connectors          []common.ComponentTranslator
}

var _ common.PipelineTranslator = (*inputPipelineTranslator)(nil)

func (t *inputPipelineTranslator) ID() pipeline.ID { return t.pipelineID }

func (t *inputPipelineTranslator) Translate(_ *confmap.Conf) (*common.ComponentTranslators, error) {
	receivers := common.NewTranslatorMap[component.Config, component.ID]()
	for _, r := range t.receivers {
		receivers.Set(r)
	}
	processors := common.NewTranslatorMap[component.Config, component.ID]()
	for _, p := range t.processors {
		processors.Set(p)
	}
	exporters := common.NewTranslatorMap[component.Config, component.ID]()
	exporters.Set(t.connectorExporterID)
	extensions := common.NewTranslatorMap[component.Config, component.ID]()
	for _, e := range t.extensions {
		extensions.Set(e)
	}
	connectors := common.NewTranslatorMap[component.Config, component.ID]()
	for _, c := range t.connectors {
		connectors.Set(c)
	}
	return &common.ComponentTranslators{
		Receivers:  receivers,
		Processors: processors,
		Exporters:  exporters,
		Extensions: extensions,
		Connectors: connectors,
	}, nil
}

// outputPipelineTranslator produces rule/default pipelines: routing connector (as receiver) → [processors] → exporter
type outputPipelineTranslator struct {
	pipelineID          pipeline.ID
	connectorReceiverID common.ComponentTranslator // routing connector appears as receiver
	processors          []common.ComponentTranslator
	exporter            common.ComponentTranslator
	extensions          []common.ComponentTranslator
}

var _ common.PipelineTranslator = (*outputPipelineTranslator)(nil)

func (t *outputPipelineTranslator) ID() pipeline.ID { return t.pipelineID }

func (t *outputPipelineTranslator) Translate(_ *confmap.Conf) (*common.ComponentTranslators, error) {
	receivers := common.NewTranslatorMap[component.Config, component.ID]()
	receivers.Set(t.connectorReceiverID)
	processors := common.NewTranslatorMap[component.Config, component.ID]()
	for _, p := range t.processors {
		processors.Set(p)
	}
	exporters := common.NewTranslatorMap[component.Config, component.ID]()
	exporters.Set(t.exporter)
	extensions := common.NewTranslatorMap[component.Config, component.ID]()
	for _, e := range t.extensions {
		extensions.Set(e)
	}
	return &common.ComponentTranslators{
		Receivers:  receivers,
		Processors: processors,
		Exporters:  exporters,
		Extensions: extensions,
	}, nil
}

// NewTranslators creates pipeline translators for the syslog configuration.
// Supports both single-object and array forms for the syslog config key.
func NewTranslators(conf *confmap.Conf) (common.PipelineTranslatorMap, error) {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()
	if conf == nil || !conf.IsSet(common.SyslogConfigKey) {
		return translators, nil
	}

	sections := normalizeSyslogSections(conf.Get(common.SyslogConfigKey))
	if err := validateUniqueListeners(sections); err != nil {
		return translators, err
	}
	for i, section := range sections {
		translators.Merge(buildSectionPipelines(section, i, conf))
	}
	return translators, nil
}

// normalizeSyslogSections converts the raw syslog config value into a slice
// of section maps, handling both single-object and array forms.
func normalizeSyslogSections(raw any) []map[string]any {
	switch v := raw.(type) {
	case map[string]any:
		return []map[string]any{v}
	case []any:
		var sections []map[string]any
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				sections = append(sections, m)
			}
		}
		return sections
	}
	return nil
}

// validateUniqueListeners checks that no listen address appears in more than
// one syslog section. Returns an error if a duplicate is found.
func validateUniqueListeners(sections []map[string]any) error {
	seen := make(map[string]int) // address → section index
	for i, section := range sections {
		for _, listener := range normalizeListeners(section) {
			addr, _ := listener["listen_address"].(string)
			if addr == "" {
				continue
			}
			if prevSection, exists := seen[addr]; exists {
				return fmt.Errorf("syslog listen address %q is defined in both section %d and section %d; each address must be unique across all sections", addr, prevSection, i)
			}
			seen[addr] = i
		}
	}
	return nil
}

// buildSectionPipelines creates all pipelines for a single syslog section.
func buildSectionPipelines(syslogConf map[string]any, sectionIdx int, conf *confmap.Conf) common.PipelineTranslatorMap {
	translators := common.NewTranslatorMap[*common.ComponentTranslators, pipeline.ID]()
	prefix := fmt.Sprintf("syslog_%d", sectionIdx)

	listeners := normalizeListeners(syslogConf)
	if len(listeners) == 0 {
		return translators
	}

	defaultLogGroupName, _ := syslogConf["log_group_name"].(string)
	defaultLogStreamName, _ := syslogConf["log_stream_name"].(string)
	if defaultLogStreamName == "" {
		defaultLogStreamName = "{hostname}"
	}
	defaultRetention := toInt64(syslogConf, "retention_in_days")

	// Build receivers from listeners
	var receiverTranslators []common.ComponentTranslator
	for _, listener := range listeners {
		listenAddress, _ := listener["listen_address"].(string)
		receiverName := deriveReceiverName(listenAddress)
		protocol, _ := listener["protocol"].(string)
		tlsConfig := toTLSConfig(listener)
		receiverTranslators = append(receiverTranslators, syslogreceiver.NewTranslator(receiverName, listenAddress, protocol, tlsConfig))
	}

	// Parse routing rules
	var rules []map[string]any
	if rawRules, ok := syslogConf["routing"].([]any); ok {
		for _, r := range rawRules {
			if rm, ok := r.(map[string]any); ok {
				rules = append(rules, rm)
			}
		}
	}

	// Build pipeline IDs
	defaultPipelineID := pipeline.NewIDWithName(pipeline.SignalLogs, prefix+"_default")
	var rulePipelineIDs []pipeline.ID
	for i := range rules {
		rulePipelineIDs = append(rulePipelineIDs, pipeline.NewIDWithName(pipeline.SignalLogs, fmt.Sprintf("%s_rule_%d", prefix, i)))
	}

	// Global filters → filter processor
	globalFilters := toFilters(syslogConf)

	// Extensions
	extensions := newExtensionTranslators()

	// No routing rules → simple single pipeline (no connector needed)
	if len(rules) == 0 {
		var processors []common.ComponentTranslator
		if len(globalFilters) > 0 {
			processors = append(processors, newFilterProcessorTranslator(prefix+"_default", globalFilters))
		}
		processors = append(processors, batchprocessor.NewTranslatorWithNameAndSection(prefix+"_default", common.LogsKey))

		translators.Set(&simplePipelineTranslator{
			pipelineID: defaultPipelineID,
			receivers:  receiverTranslators,
			processors: processors,
			exporter:   newCWLExporterTranslator(prefix+"_default", defaultLogGroupName, defaultLogStreamName, defaultRetention),
			extensions: extensions,
		})
		return translators
	}

	// Build routing table entries
	var tableEntries []routingTableEntry
	for i, rule := range rules {
		matchMap, _ := rule["match"].(map[string]any)
		condition := buildOTTLCondition(matchMap)
		if condition != "" {
			tableEntries = append(tableEntries, routingTableEntry{
				condition: condition,
				pipelines: []pipeline.ID{rulePipelineIDs[i]},
			})
		}
	}

	// Create routing connector translator
	routingTranslator := newRoutingConnectorTranslator(prefix, []pipeline.ID{defaultPipelineID}, tableEntries)

	// Global filters → filter processor for input pipeline
	inName := prefix + "_in"
	var inputProcessors []common.ComponentTranslator
	if len(globalFilters) > 0 {
		inputProcessors = append(inputProcessors, newFilterProcessorTranslator(inName, globalFilters))
	}

	// Input pipeline: receivers → [filter] → routing connector (as exporter)
	translators.Set(&inputPipelineTranslator{
		pipelineID:          pipeline.NewIDWithName(pipeline.SignalLogs, inName),
		receivers:           receiverTranslators,
		processors:          inputProcessors,
		connectorExporterID: routingTranslator,
		extensions:          extensions,
		connectors:          []common.ComponentTranslator{routingTranslator},
	})

	// Rule pipelines
	for i, rule := range rules {
		ruleLogGroup, _ := rule["log_group_name"].(string)
		if ruleLogGroup == "" {
			ruleLogGroup = defaultLogGroupName
		}
		ruleLogStream, _ := rule["log_stream_name"].(string)
		if ruleLogStream == "" {
			ruleLogStream = defaultLogStreamName
		}
		ruleRetention := toInt64(rule, "retention_in_days")
		if ruleRetention == 0 {
			ruleRetention = defaultRetention
		}

		pipelineName := fmt.Sprintf("%s_rule_%d", prefix, i)
		var processors []common.ComponentTranslator

		// Per-rule filters
		ruleFilters := toFilters(rule)
		if len(ruleFilters) > 0 {
			processors = append(processors, newFilterProcessorTranslator(pipelineName, ruleFilters))
		}

		processors = append(processors, batchprocessor.NewTranslatorWithNameAndSection(pipelineName, common.LogsKey))

		translators.Set(&outputPipelineTranslator{
			pipelineID:          rulePipelineIDs[i],
			connectorReceiverID: routingTranslator,
			processors:          processors,
			exporter:            newCWLExporterTranslator(pipelineName, ruleLogGroup, ruleLogStream, ruleRetention),
			extensions:          extensions,
		})
	}

	// Default pipeline
	defaultPipelineName := prefix + "_default"
	var defaultProcessors []common.ComponentTranslator
	defaultProcessors = append(defaultProcessors, batchprocessor.NewTranslatorWithNameAndSection(defaultPipelineName, common.LogsKey))

	translators.Set(&outputPipelineTranslator{
		pipelineID:          defaultPipelineID,
		connectorReceiverID: routingTranslator,
		processors:          defaultProcessors,
		exporter:            newCWLExporterTranslator(defaultPipelineName, defaultLogGroupName, defaultLogStreamName, defaultRetention),
		extensions:          extensions,
	})

	return translators
}

func newExtensionTranslators() []common.ComponentTranslator {
	return []common.ComponentTranslator{
		agenthealth.NewTranslator(agenthealth.LogsName, []string{agenthealth.OperationPutLogEvents}),
		agenthealth.NewTranslatorWithStatusCode(agenthealth.StatusCodeName, nil, true),
	}
}

// normalizeListeners converts the config into a slice of listener maps.
// Supports both "listeners" array and "listen_address" shorthand.
func normalizeListeners(syslogConf map[string]any) []map[string]any {
	if rawListeners, ok := syslogConf["listeners"].([]any); ok {
		var listeners []map[string]any
		for _, l := range rawListeners {
			if lm, ok := l.(map[string]any); ok {
				listeners = append(listeners, lm)
			}
		}
		return listeners
	}
	// Shorthand: single listener from top-level fields
	if addr, ok := syslogConf["listen_address"].(string); ok && addr != "" {
		listener := map[string]any{"listen_address": addr}
		if p, ok := syslogConf["protocol"].(string); ok {
			listener["protocol"] = p
		}
		if t, ok := syslogConf["tls"]; ok {
			listener["tls"] = t
		}
		return []map[string]any{listener}
	}
	return nil
}

// buildOTTLCondition converts a match map to an OTTL condition string.
func buildOTTLCondition(matchMap map[string]any) string {
	if len(matchMap) == 0 {
		return ""
	}
	var conditions []string
	if hostname, ok := matchMap["hostname"].(string); ok && hostname != "" {
		conditions = append(conditions, buildAttributeCondition("hostname", hostname))
	}
	if appName, ok := matchMap["app_name"].(string); ok && appName != "" {
		conditions = append(conditions, buildAttributeCondition("app_name", appName))
	}
	if facility, ok := matchMap["facility"]; ok {
		var facStr string
		switch f := facility.(type) {
		case float64:
			facStr = fmt.Sprintf("%d", int(f))
		case int:
			facStr = fmt.Sprintf("%d", f)
		default:
			facStr = fmt.Sprintf("%v", f)
		}
		conditions = append(conditions, fmt.Sprintf(`attributes["facility"] == %s`, facStr))
	}
	return strings.Join(conditions, " and ")
}

func buildAttributeCondition(attr, value string) string {
	escaped := escapeOTTL(value)
	if isGlobPattern(value) {
		return fmt.Sprintf(`IsMatch(attributes["%s"], "%s")`, attr, escaped)
	}
	return fmt.Sprintf(`attributes["%s"] == "%s"`, attr, escaped)
}

// isGlobPattern detects if a string contains glob characters.
func isGlobPattern(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func deriveReceiverName(listenAddress string) string {
	name := strings.ReplaceAll(listenAddress, "://", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, ":", "_")
	// IPv6 addresses are wrapped in brackets (e.g. [::1]); strip them so the
	// resulting component name contains only alphanumerics and underscores.
	name = strings.ReplaceAll(name, "[", "")
	name = strings.ReplaceAll(name, "]", "")
	return name
}

func toFilters(m map[string]any) []filter {
	rawFilters, ok := m["filters"].([]any)
	if !ok {
		return nil
	}
	var filters []filter
	for _, rf := range rawFilters {
		fm, ok := rf.(map[string]any)
		if !ok {
			continue
		}
		f := filter{}
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

type filter struct {
	Type       string
	Expression string
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
