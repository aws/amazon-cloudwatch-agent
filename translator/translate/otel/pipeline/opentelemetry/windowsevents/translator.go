// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package windowsevents

import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/hash"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/forward"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/filestorage"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourceprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/transformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/windowseventlog"
)

// eventLevelNumbers maps config level names to Windows Event Log numeric levels.
var eventLevelNumbers = map[string][]string{
	"CRITICAL":    {"1"},
	"ERROR":       {"2"},
	"WARNING":     {"3"},
	"INFORMATION": {"4", "0"},
	"VERBOSE":     {"5"},
}

type eventEntry struct {
	index         int
	channel       string
	format        string
	resource      map[string]string
	logGroupName  string
	logStreamName string
	eventLevels   []string
	eventIDs      []int
}

func (e eventEntry) raw() bool {
	return e.format == "xml"
}

func (e eventEntry) name() string {
	return fmt.Sprintf("%s_%d", common.SanitizeName(e.channel), e.index)
}

func (e eventEntry) receiverName() string {
	return fmt.Sprintf("%s_%s", common.SanitizeName(e.channel), e.receiverHash())
}

func (e eventEntry) receiverHash() string {
	return hash.HashName(fmt.Sprintf("%s\x00%s\x00%v\x00%v", e.channel, e.format, e.eventLevels, e.eventIDs))
}

func (e eventEntry) routingAttributes() map[string]string {
	if e.logGroupName == "" && e.logStreamName == "" {
		return nil
	}
	attrs := make(map[string]string)
	if e.logGroupName != "" {
		attrs["aws.log.group.name"] = e.logGroupName
	}
	if e.logStreamName != "" {
		attrs["aws.log.stream.name"] = e.logStreamName
	}
	return attrs
}

const ignoreOlderThanTwoWeeksMs = int64(14*24*time.Hour) / int64(time.Millisecond)

// queryXML builds a Windows Event Log XPath query XML for native OS-level filtering.
// Always includes a 2-week time cutoff to prevent replaying stale events on restart.
func (e eventEntry) queryXML() string {
	var filters []string

	if len(e.eventLevels) > 0 {
		var levelChecks []string
		for _, level := range e.eventLevels {
			if nums, ok := eventLevelNumbers[level]; ok {
				for _, n := range nums {
					levelChecks = append(levelChecks, fmt.Sprintf("Level='%s'", n))
				}
			}
		}
		if len(levelChecks) > 0 {
			filters = append(filters, "("+strings.Join(levelChecks, " or ")+")")
		}
	}

	if len(e.eventIDs) > 0 {
		var idChecks []string
		for _, id := range e.eventIDs {
			idChecks = append(idChecks, fmt.Sprintf("EventID='%d'", id))
		}
		if len(idChecks) > 0 {
			filters = append(filters, "("+strings.Join(idChecks, " or ")+")")
		}
	}

	filters = append(filters, fmt.Sprintf("TimeCreated[timediff(@SystemTime) &lt;= %d]", ignoreOlderThanTwoWeeksMs))

	return fmt.Sprintf(`<QueryList><Query Id="0"><Select Path="%s">*[System[%s]]</Select></Query></QueryList>`,
		e.channel, strings.Join(filters, " and "))
}

type windowsEventsPipelineTranslator struct {
	entry eventEntry
}

var _ common.PipelineTranslator = (*windowsEventsPipelineTranslator)(nil)

func (t *windowsEventsPipelineTranslator) ID() pipeline.ID {
	return pipeline.NewIDWithName(pipeline.SignalLogs, "windows_events_"+t.entry.name())
}

func (t *windowsEventsPipelineTranslator) Translate(_ *confmap.Conf) (*common.ComponentTranslators, error) {
	fwdConnector := forward.NewTranslator(common.OpenTelemetryKey)

	query := t.entry.queryXML()
	receivers := common.NewTranslatorMap[component.Config, component.ID]()
	receivers.Set(windowseventlog.NewTranslator(t.entry.receiverName(), t.entry.channel, t.entry.raw(), query, t.entry.resource))

	processors := common.NewTranslatorMap[component.Config, component.ID]()

	if attrs := t.entry.routingAttributes(); len(attrs) > 0 {
		processors.Set(resourceprocessor.NewTranslator(
			common.WithName("windows_events_"+t.entry.name()),
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
