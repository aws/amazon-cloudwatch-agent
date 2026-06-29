// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windowsevents

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/forward"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/filterprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/transformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/windowseventlog"
)

// severityNumbers maps config level names to OTel severity numbers.
// Derived from pkg/stanza/operator/input/windows/xml.go parseSeverity():
//
//	Level "1"/Critical → Fatal(21), "2"/Error → Error(17),
//	"3"/Warning → Warn(13), "4"/Information → Info(9), default → Default(0)
//
// TODO: Replace with upstream Query XML filtering when collector is bumped past v0.124.
var severityNumbers = map[string]int{
	"CRITICAL":    21,
	"ERROR":       17,
	"WARNING":     13,
	"INFORMATION": 9,
	"VERBOSE":     0,
}

type eventEntry struct {
	name        string
	channel     string
	raw         bool
	resource    map[string]string
	eventLevels []string
	eventIDs    []int
}

func (e eventEntry) filterCondition() string {
	var parts []string

	if len(e.eventLevels) > 0 {
		var severityChecks []string
		for _, level := range e.eventLevels {
			if sev, ok := severityNumbers[level]; ok {
				severityChecks = append(severityChecks, fmt.Sprintf("severity_number == %d", sev))
			}
		}
		if len(severityChecks) > 0 {
			parts = append(parts, "("+strings.Join(severityChecks, " or ")+")")
		}
	}

	if len(e.eventIDs) > 0 {
		var idChecks []string
		for _, id := range e.eventIDs {
			idChecks = append(idChecks, fmt.Sprintf("body[\"event_id\"][\"id\"] == %d", id))
		}
		if len(idChecks) > 0 {
			parts = append(parts, "("+strings.Join(idChecks, " or ")+")")
		}
	}

	if len(parts) == 0 {
		return ""
	}

	// Drop records that DON'T match the filter (filter processor drops when condition is true)
	return "not(" + strings.Join(parts, " and ") + ")"
}

type windowsEventsPipelineTranslator struct {
	entry eventEntry
}

var _ common.PipelineTranslator = (*windowsEventsPipelineTranslator)(nil)

func (t *windowsEventsPipelineTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalLogs, "windows_events_"+t.entry.name)
}

func (t *windowsEventsPipelineTranslator) Translate(_ *confmap.Conf) (*common.ComponentTranslators, error) {
	fwdConnector := forward.NewTranslator(common.OpenTelemetryKey)

	receivers := common.NewTranslatorMap[component.Config, component.ID]()
	receivers.Set(windowseventlog.NewTranslator(t.entry.name, t.entry.channel, t.entry.raw, t.entry.resource))

	processors := common.NewTranslatorMap[component.Config, component.ID]()
	processors.Set(transformprocessor.NewTranslatorWithName("windows_events_scope_"+t.entry.name,
		transformprocessor.WithErrorMode("ignore"),
		transformprocessor.WithScopeStatements([]string{
			`set(scope.attributes["cloudwatch.source"], "cloudwatch-agent")`,
			`set(scope.attributes["cloudwatch.solution"], "otel-windows-events")`,
		}),
	))

	// TODO: Replace with upstream Query XML filtering when collector is bumped past v0.124.
	condition := t.entry.filterCondition()
	if condition != "" {
		processors.Set(filterprocessor.NewTranslatorWithLogCondition("windows_events_"+t.entry.name, condition))
	}

	return &common.ComponentTranslators{
		Receivers:  receivers,
		Processors: processors,
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
	}, nil
}
