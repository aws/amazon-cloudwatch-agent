// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package databaseinsights

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/count"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/forward"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/connector/signaltometrics"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/filterprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetection"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/transformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/filelog"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/postgresql"
)

type dbiPipelineType int

const (
	dbiMetrics dbiPipelineType = iota
	dbiLogToMetrics
	dbiRawEvents
	dbiServerLogs
)

// postgresLogSeverityLevels are the severity keywords PostgreSQL emits in stderr log
// lines. Kept alongside the mapping below so the regex and mapping stay together.
var postgresLogSeverityLevels = []string{
	"LOG", "ERROR", "WARNING", "FATAL", "PANIC", `DEBUG\d?`, "INFO", "NOTICE", "STATEMENT",
}

// postgresLogSeverityMapping maps PostgreSQL log severity keywords to OTEL severity
// levels for the filelog severity operator. Kept here (not in the generic filelog
// translator) because these levels are PostgreSQL-specific.
var postgresLogSeverityMapping = map[string]any{
	"debug": []string{"DEBUG", "DEBUG1", "DEBUG2", "DEBUG3", "DEBUG4", "DEBUG5"},
	"info":  []string{"LOG", "INFO", "NOTICE", "STATEMENT"},
	"warn":  "WARNING",
	"error": "ERROR",
	"fatal": []string{"FATAL", "PANIC"},
}

// buildPostgresSeverityPattern builds the regex that extracts the severity from a
// PostgreSQL stderr log line of the form "... [<pid>] <SEVERITY>: ...". The required
// (?P<severity>...) named capture group is assembled from postgresLogSeverityLevels,
// mirroring how the timestamp regex is built rather than inlining a raw pattern.
func buildPostgresSeverityPattern() string {
	return `\[\d+\]\s*(?P<severity>` + strings.Join(postgresLogSeverityLevels, "|") + `):`
}

type dbiTranslator struct {
	pipelineType  dbiPipelineType
	instanceIndex int
	cfg           dbiInstanceConfig
}

var _ common.PipelineTranslator = (*dbiTranslator)(nil)

func (t *dbiTranslator) ID() pipeline.ID {
	idx := strconv.Itoa(t.instanceIndex)
	switch t.pipelineType {
	case dbiMetrics:
		return pipeline.NewIDWithName(pipeline.SignalMetrics, "dbi_postgresql_"+idx)
	case dbiLogToMetrics:
		return pipeline.NewIDWithName(pipeline.SignalLogs, "dbi_postgresql_"+idx)
	case dbiRawEvents:
		return pipeline.NewIDWithName(pipeline.SignalLogs, "dbi_postgresql_rawevents_"+idx)
	case dbiServerLogs:
		return pipeline.NewIDWithName(pipeline.SignalLogs, "dbi_postgresql_serverlogs_"+idx)
	}
	return pipeline.NewID(pipeline.SignalMetrics)
}

func (t *dbiTranslator) Translate(_ *confmap.Conf) (*common.ComponentTranslators, error) {
	if err := validateOttlSafe("username", t.cfg.username); err != nil {
		return nil, err
	}
	if err := validateOttlSafe("instance_name", t.cfg.instanceName); err != nil {
		return nil, err
	}
	switch t.pipelineType {
	case dbiMetrics:
		return t.translateMetrics()
	case dbiLogToMetrics:
		return t.translateLogToMetrics()
	case dbiRawEvents:
		return t.translateRawEvents()
	case dbiServerLogs:
		return t.translateServerLogs()
	}
	return nil, fmt.Errorf("unknown DBI pipeline type: %d", t.pipelineType)
}

func (t *dbiTranslator) translateMetrics() (*common.ComponentTranslators, error) {
	idx := strconv.Itoa(t.instanceIndex)
	fwd := forward.NewTranslator(common.OpenTelemetryKey)
	countConn := count.NewTranslator(common.DbiConnectorDbload)
	s2mConn := signaltometrics.NewTranslator(common.DbiConnectorTopsql)

	return &common.ComponentTranslators{
		Receivers: common.NewTranslatorMap[component.Config, component.ID](t.pgReceiver("metrics"), countConn, s2mConn),
		Processors: common.NewTranslatorMap[component.Config, component.ID](
			t.scopeTransform(),
			transformprocessor.NewTranslatorWithName(common.DbiTransformResource+"_"+idx, transformprocessor.WithMetricResourceStatements(t.resourceStatements())),
			transformprocessor.NewTranslatorWithName(common.DbiTransformFixStartTime)),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](fwd),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwd, countConn, s2mConn),
	}, nil
}

func (t *dbiTranslator) translateLogToMetrics() (*common.ComponentTranslators, error) {
	countConn := count.NewTranslator(common.DbiConnectorDbload)
	s2mConn := signaltometrics.NewTranslator(common.DbiConnectorTopsql)

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](t.pgReceiver("metrics")),
		Processors: common.NewTranslatorMap[component.Config, component.ID](t.excludeMonitorFilter()),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](countConn, s2mConn),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](countConn, s2mConn),
	}, nil
}

func (t *dbiTranslator) translateRawEvents() (*common.ComponentTranslators, error) {
	idx := strconv.Itoa(t.instanceIndex)
	fwd := forward.NewTranslator(common.OpenTelemetryKey)

	return &common.ComponentTranslators{
		Receivers: common.NewTranslatorMap[component.Config, component.ID](t.pgReceiver("events", postgresql.WithQuerySampleInterval(60*time.Second))),
		Processors: common.NewTranslatorMap[component.Config, component.ID](
			t.excludeMonitorFilter(),
			t.scopeTransform(),
			resourcedetection.NewTranslator(resourcedetection.WithName(common.OpenTelemetryKey)),
			transformprocessor.NewTranslatorWithName(
				common.DbiTransformResource+"_"+idx,
				transformprocessor.WithMetricResourceStatements(t.resourceStatements()),
				transformprocessor.WithLogResourceStatements(t.resourceStatements()),
			),
			transformprocessor.NewTranslatorWithName(
				common.DbiTransformLogs+"_raw-events_"+idx,
				transformprocessor.WithLogResourceStatements(t.logStatements("raw-events")),
			),
		),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](fwd),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwd),
	}, nil
}

func (t *dbiTranslator) translateServerLogs() (*common.ComponentTranslators, error) {
	idx := strconv.Itoa(t.instanceIndex)
	fwd := forward.NewTranslator(common.OpenTelemetryKey)

	// NOTE: timestamp parsing assumes the PostgreSQL instance logs in UTC. The %Z token
	// only matches 3-letter zone abbreviations and the gotime layout parses them against a
	// UTC location, so timestamps from a non-UTC instance would be silently offset. DBI
	// therefore requires the instance to be configured with log_timezone = 'UTC'.
	return &common.ComponentTranslators{
		Receivers: common.NewTranslatorMap[component.Config, component.ID](
			filelog.NewTranslator(filelog.WithNamePrefix("postgresql"),
				filelog.WithIndex(t.instanceIndex), filelog.WithFilePath(t.cfg.logFilePath),
				filelog.WithMultilinePattern(`^\d{4}-\d{2}-\d{2}`),
				filelog.WithTimestampFormat("%Y-%m-%d %H:%M:%S.%f %Z", "UTC"),
				filelog.WithSeverityPattern(buildPostgresSeverityPattern()),
				filelog.WithSeverityMapping(postgresLogSeverityMapping)),
		),
		Processors: common.NewTranslatorMap[component.Config, component.ID](
			t.scopeTransform(),
			resourcedetection.NewTranslator(resourcedetection.WithName(common.OpenTelemetryKey)),
			transformprocessor.NewTranslatorWithName(
				common.DbiTransformResource+"_"+idx,
				transformprocessor.WithMetricResourceStatements(t.resourceStatements()),
				transformprocessor.WithLogResourceStatements(t.resourceStatements()),
			),
			transformprocessor.NewTranslatorWithName(
				common.DbiTransformLogs+"_server-logs_"+idx,
				transformprocessor.WithLogResourceStatements(t.logStatements("server-logs")),
				// Drop the parsed attributes once promoted to the record's timestamp and
				// severity fields, mirroring the files pipeline's timestamp cleanup, so they
				// are not duplicated in the emitted log attributes.
				transformprocessor.WithLogContextStatements([]string{
					`delete_key(attributes, "timestamp")`,
					`delete_key(attributes, "severity")`,
				}),
			),
		),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](fwd),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwd),
	}, nil
}

func (t *dbiTranslator) pgReceiver(name string, extraOpts ...postgresql.Option) common.ComponentTranslator {
	opts := []postgresql.Option{
		postgresql.WithName(name),
		postgresql.WithIndex(t.instanceIndex),
		postgresql.WithEndpoint(t.cfg.endpoint),
		postgresql.WithUsername(t.cfg.username),
		postgresql.WithPassfile(t.cfg.passfile),
		postgresql.WithCAFile(t.cfg.caFile),
		postgresql.WithIsLocalhost(t.cfg.isLocalhost),
	}
	opts = append(opts, extraOpts...)
	return postgresql.NewTranslator(opts...)
}

func (t *dbiTranslator) excludeMonitorFilter() common.ComponentTranslator {
	idx := strconv.Itoa(t.instanceIndex)
	condition := fmt.Sprintf(`attributes["user.name"] == "%s" or attributes["postgresql.rolname"] == "%s"`, t.cfg.username, t.cfg.username)
	return filterprocessor.NewTranslatorWithLogCondition(common.DbiFilterExcludeMonitor+"_"+idx, condition, common.OTTLErrorModePropagate)
}

func (t *dbiTranslator) scopeTransform() common.ComponentTranslator {
	return transformprocessor.NewTranslatorWithName("dbi_scope",
		transformprocessor.WithErrorMode("ignore"),
		transformprocessor.WithMetricScopeStatements(common.ScopeStatementsForSolution("otel-database-insights")),
		transformprocessor.WithLogScopeStatements(common.ScopeStatementsForSolution("otel-database-insights")),
	)
}

func (t *dbiTranslator) resourceStatements() []string {
	return []string{
		`set(resource.attributes["db.system.name"], "postgresql")`,
		fmt.Sprintf(`set(resource.attributes["db.instance.name"], "%s")`, t.cfg.instanceName),
	}
}

func (t *dbiTranslator) logStatements(destination string) []string {
	return []string{
		fmt.Sprintf(`set(resource.attributes["aws.log.group.name"], "/aws/self-managed-database-insights/postgresql/%s")`, destination),
		fmt.Sprintf(`set(resource.attributes["aws.log.stream.name"], Concat([resource.attributes["host.id"], "%s"], "/"))`, t.cfg.instanceName),
	}
}
