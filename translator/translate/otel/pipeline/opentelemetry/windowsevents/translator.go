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
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/filestorage"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/filterprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourceprocessor"
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
	name          string
	receiverName  string
	channel       string
	raw           bool
	resource      map[string]string
	logGroupName  string
	logStreamName string
	eventLevels   []string
	eventIDs      []int
}

func (e eventEntry) routingAttributes() map[string]string {
	attrs := make(map[string]string)
	if e.logGroupName != "" {
		attrs["aws.log.group.name"] = e.logGroupName
	}
	if e.logStreamName != "" {
		attrs["aws.log.stream.name"] = e.logStreamName
	}
	return attrs
}

// filterCondition builds an OTTL drop condition. When both levels and IDs are present, they are ANDed.
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
	if t.entry.raw && len(t.entry.eventIDs) > 0 {
		return nil, fmt.Errorf("event_ids filtering is not supported with event_format \"xml\" for channel %q", t.entry.channel)
	}

	fwdConnector := forward.NewTranslator(common.OpenTelemetryKey)

	receivers := common.NewTranslatorMap[component.Config, component.ID]()
	receivers.Set(windowseventlog.NewTranslator(t.entry.receiverName, t.entry.channel, t.entry.raw, t.entry.resource))

	processors := common.NewTranslatorMap[component.Config, component.ID]()

	// TODO: Replace with upstream Query XML filtering when collector is bumped past v0.124.
	condition := t.entry.filterCondition()
	if condition != "" {
		processors.Set(filterprocessor.NewTranslatorWithLogCondition("windows_events_"+t.entry.name, condition, common.OTTLErrorModeIgnore))
	}

	if attrs := t.entry.routingAttributes(); len(attrs) > 0 {
		processors.Set(resourceprocessor.NewTranslator(
			common.WithName("windows_events_"+t.entry.name),
			resourceprocessor.WithAttributes(attrs),
		))
	}
	processors.Set(transformprocessor.NewTranslatorWithName("windows_events_scope",
		transformprocessor.WithErrorMode(common.OTTLErrorModeIgnore),
		transformprocessor.WithLogScopeStatements(common.ScopeStatementsForSolution("otel-windows-events")),
	))

	return &common.ComponentTranslators{
		Receivers:  receivers,
		Processors: processors,
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](filestorage.NewTranslator()),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwdConnector),
	}, nil
}
