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
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/groupbyattrsprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/resourcedetection"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/processor/transformprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/filelog"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/mysql"
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

// mysqlLogSeverityLevels are the severity labels MySQL emits in the classic error log
// (per MySQL worklog #10942): Note/Warning/ERROR, plus System for force-printed events.
// Kept alongside the mapping below so the regex and mapping stay together.
var mysqlLogSeverityLevels = []string{"System", "Warning", "Note", "ERROR"}

// mysqlLogSeverityMapping maps MySQL error-log labels to OTEL severity levels for the
// filelog severity operator. Kept here (not in the generic filelog translator) because
// these labels are MySQL-specific. System and Note are informational; there is no
// MySQL debug/fatal label in the classic error log.
var mysqlLogSeverityMapping = map[string]any{
	"info":  []string{"System", "Note"},
	"warn":  "Warning",
	"error": "ERROR",
}

// buildMysqlSeverityPattern builds the regex that extracts the severity from a MySQL
// error-log line of the form "<timestamp> <thread_id> [<severity>] [MY-######] ...".
// Anchoring on the numeric thread id before the bracket avoids matching the later
// error-code bracket (e.g. "[MY-010116]"). The required (?P<severity>...) named capture
// group is assembled from mysqlLogSeverityLevels.
func buildMysqlSeverityPattern() string {
	return `\s\d+\s+\[(?P<severity>` + strings.Join(mysqlLogSeverityLevels, "|") + `)\]`
}

// dbiTranslator generates DBI pipelines for a single database instance. The
// engine (carried on cfg) selects engine-specific receivers, connector configs,
// resource attributes, and log group paths. Component IDs are index-based so
// PostgreSQL and MySQL instances share one consistent naming scheme.
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
		return pipeline.NewIDWithName(pipeline.SignalMetrics, "dbi_"+t.cfg.engine+"_"+idx)
	case dbiLogToMetrics:
		return pipeline.NewIDWithName(pipeline.SignalLogs, "dbi_"+t.cfg.engine+"_"+idx)
	case dbiRawEvents:
		return pipeline.NewIDWithName(pipeline.SignalLogs, "dbi_"+t.cfg.engine+"_rawevents_"+idx)
	case dbiServerLogs:
		return pipeline.NewIDWithName(pipeline.SignalLogs, "dbi_"+t.cfg.engine+"_serverlogs_"+idx)
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
	countConn := count.NewTranslator(common.DbiConnectorDbload+"_"+t.cfg.engine, t.cfg.engine)
	s2mConn := signaltometrics.NewTranslator(common.DbiConnectorTopsql+"_"+t.cfg.engine, t.cfg.engine)

	return &common.ComponentTranslators{
		Receivers: common.NewTranslatorMap[component.Config, component.ID](t.receiver("metrics"), countConn, s2mConn),
		Processors: common.NewTranslatorMap[component.Config, component.ID](
			t.scopeTransform(),
			transformprocessor.NewTranslatorWithName(common.DbiTransformResource+"_"+t.cfg.engine+"_"+idx, transformprocessor.WithMetricResourceStatements(t.resourceStatements())),
			transformprocessor.NewTranslatorWithName(common.DbiTransformFixStartTime+"_"+t.cfg.engine, transformprocessor.WithDbiFixStartTime(t.cfg.engine))),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](fwd),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwd, countConn, s2mConn),
	}, nil
}

func (t *dbiTranslator) translateLogToMetrics() (*common.ComponentTranslators, error) {
	countConn := count.NewTranslator(common.DbiConnectorDbload+"_"+t.cfg.engine, t.cfg.engine)
	s2mConn := signaltometrics.NewTranslator(common.DbiConnectorTopsql+"_"+t.cfg.engine, t.cfg.engine)

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](t.receiver("metrics")),
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
		Receivers: common.NewTranslatorMap[component.Config, component.ID](t.receiver("events")),
		Processors: common.NewTranslatorMap[component.Config, component.ID](
			t.excludeMonitorFilter(),
			t.scopeTransform(),
			resourcedetection.NewTranslator(resourcedetection.WithName(common.OpenTelemetryKey)),
			transformprocessor.NewTranslatorWithName(
				common.DbiTransformResource+"_"+t.cfg.engine+"_"+idx,
				transformprocessor.WithMetricResourceStatements(t.resourceStatements()),
				transformprocessor.WithLogResourceStatements(t.resourceStatements()),
			),
			transformprocessor.NewTranslatorWithName(
				common.DbiTransformLogs+"_"+t.cfg.engine+"_raw-events_"+idx,
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

	// NOTE: timestamp parsing assumes the instance logs in UTC. Both engines' timestamp
	// layouts parse against a UTC location, so timestamps from a non-UTC instance would be
	// silently offset. DBI therefore requires the instance to emit UTC timestamps in its
	// logs (PostgreSQL: log_timezone = 'UTC'; MySQL: log_timestamps = UTC, which is the
	// default).
	return &common.ComponentTranslators{
		Receivers: common.NewTranslatorMap[component.Config, component.ID](t.serverLogReceiver()),
		Processors: common.NewTranslatorMap[component.Config, component.ID](
			t.scopeTransform(),
			resourcedetection.NewTranslator(resourcedetection.WithName(common.OpenTelemetryKey)),
			// Promote log.file.name to a resource attribute, reusing the files pipeline's
			// shared groupbyattrs instance so server-log parsing/grouping is consistent
			// with regular file logs.
			groupbyattrsprocessor.NewTranslatorWithName(common.FilesKey, "log.file.name"),
			transformprocessor.NewTranslatorWithName(
				common.DbiTransformResource+"_"+t.cfg.engine+"_"+idx,
				transformprocessor.WithMetricResourceStatements(t.resourceStatements()),
				transformprocessor.WithLogResourceStatements(t.resourceStatements()),
			),
			transformprocessor.NewTranslatorWithName(
				common.DbiTransformLogs+"_"+t.cfg.engine+"_server-logs_"+idx,
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

// serverLogReceiver builds the engine-specific filelog receiver for server logs.
// Both engines get multiline grouping plus timestamp and severity parsing, but the
// timestamp layout and severity vocabulary differ by engine (PostgreSQL stderr format
// vs MySQL classic error-log format).
func (t *dbiTranslator) serverLogReceiver() common.ComponentTranslator {
	opts := []filelog.Option{
		filelog.WithNamePrefix(t.cfg.engine),
		filelog.WithIndex(t.instanceIndex),
		filelog.WithFilePath(t.cfg.logFilePath),
		filelog.WithMultilinePattern(`^\d{4}-\d{2}-\d{2}`),
	}
	switch t.cfg.engine {
	case common.PostgreSQLKey:
		opts = append(opts,
			filelog.WithTimestampFormat("%Y-%m-%d %H:%M:%S.%f %Z", "UTC"),
			filelog.WithSeverityPattern(buildPostgresSeverityPattern()),
			filelog.WithSeverityMapping(postgresLogSeverityMapping),
		)
	case common.MySQLKey:
		// MySQL 8.0+ error log timestamps are ISO 8601 UTC, e.g.
		// "2026-07-20T15:27:47.123456Z" (T separator, trailing literal Z).
		opts = append(opts,
			filelog.WithTimestampFormat("%Y-%m-%dT%H:%M:%S.%fZ", "UTC"),
			filelog.WithSeverityPattern(buildMysqlSeverityPattern()),
			filelog.WithSeverityMapping(mysqlLogSeverityMapping),
		)
	}
	return filelog.NewTranslator(opts...)
}

// receiver builds the engine-specific receiver translator. name is "metrics" or
// "events"; for PostgreSQL events we override the query sample interval.
func (t *dbiTranslator) receiver(name string) common.ComponentTranslator {
	if t.cfg.engine == common.MySQLKey {
		return mysql.NewTranslator(
			mysql.WithName(name),
			mysql.WithIndex(t.instanceIndex),
			mysql.WithEndpoint(t.cfg.endpoint),
			mysql.WithUsername(t.cfg.username),
			mysql.WithPassfile(t.cfg.passfile),
		)
	}

	opts := []postgresql.Option{
		postgresql.WithName(name),
		postgresql.WithIndex(t.instanceIndex),
		postgresql.WithEndpoint(t.cfg.endpoint),
		postgresql.WithUsername(t.cfg.username),
		postgresql.WithPassfile(t.cfg.passfile),
		postgresql.WithCAFile(t.cfg.caFile),
		postgresql.WithIsLocalhost(t.cfg.isLocalhost),
	}
	if name == "events" {
		opts = append(opts, postgresql.WithQuerySampleInterval(60*time.Second))
	}
	return postgresql.NewTranslator(opts...)
}

func (t *dbiTranslator) excludeMonitorFilter() common.ComponentTranslator {
	idx := strconv.Itoa(t.instanceIndex)
	condition := fmt.Sprintf(`attributes["user.name"] == "%s"`, t.cfg.username)
	if t.cfg.engine == common.PostgreSQLKey {
		condition = fmt.Sprintf(`attributes["user.name"] == "%s" or attributes["postgresql.rolname"] == "%s"`, t.cfg.username, t.cfg.username)
	}
	return filterprocessor.NewTranslatorWithLogCondition(common.DbiFilterExcludeMonitor+"_"+t.cfg.engine+"_"+idx, condition, common.OTTLErrorModePropagate)
}

func (t *dbiTranslator) scopeTransform() common.ComponentTranslator {
	idx := strconv.Itoa(t.instanceIndex)
	return transformprocessor.NewTranslatorWithName("dbi_scope_"+t.cfg.engine+"_"+idx,
		transformprocessor.WithErrorMode("ignore"),
		transformprocessor.WithMetricScopeStatements(common.ScopeStatementsForSolution("otel-database-insights")),
		transformprocessor.WithLogScopeStatements(common.ScopeStatementsForSolution("otel-database-insights")),
	)
}

func (t *dbiTranslator) resourceStatements() []string {
	return []string{
		fmt.Sprintf(`set(resource.attributes["db.system.name"], "%s")`, t.cfg.engine),
		fmt.Sprintf(`set(resource.attributes["db.instance.name"], "%s")`, t.cfg.instanceName),
	}
}

func (t *dbiTranslator) logStatements(destination string) []string {
	return []string{
		fmt.Sprintf(`set(resource.attributes["aws.log.group.name"], "/aws/self-managed-database-insights/%s/%s")`, t.cfg.engine, destination),
		fmt.Sprintf(`set(resource.attributes["aws.log.stream.name"], Concat([resource.attributes["host.id"], "%s"], "/"))`, t.cfg.instanceName),
	}
}
