// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package databaseinsights

import (
	"fmt"
	"strconv"

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
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/mysql"
)

type dbiMysqlTranslator struct {
	pipelineType  dbiPipelineType
	instanceIndex int
	cfg           dbiInstanceConfig
}

var _ common.PipelineTranslator = (*dbiMysqlTranslator)(nil)

func (t *dbiMysqlTranslator) ID() pipeline.ID {
	idx := strconv.Itoa(t.instanceIndex)
	switch t.pipelineType {
	case dbiMetrics:
		return pipeline.NewIDWithName(pipeline.SignalMetrics, "dbi_mysql_"+idx)
	case dbiLogToMetrics:
		return pipeline.NewIDWithName(pipeline.SignalLogs, "dbi_mysql_"+idx)
	case dbiRawEvents:
		return pipeline.NewIDWithName(pipeline.SignalLogs, "dbi_mysql_rawevents_"+idx)
	case dbiServerLogs:
		return pipeline.NewIDWithName(pipeline.SignalLogs, "dbi_mysql_serverlogs_"+idx)
	}
	return pipeline.NewID(pipeline.SignalMetrics)
}

func (t *dbiMysqlTranslator) Translate(_ *confmap.Conf) (*common.ComponentTranslators, error) {
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
	return nil, fmt.Errorf("unknown DBI MySQL pipeline type: %d", t.pipelineType)
}

func (t *dbiMysqlTranslator) translateMetrics() (*common.ComponentTranslators, error) {
	idx := strconv.Itoa(t.instanceIndex)
	fwd := forward.NewTranslator(common.OpenTelemetryKey)
	countConn := count.NewTranslator(common.DbiConnectorDbloadMysql)
	s2mConn := signaltometrics.NewTranslator(common.DbiConnectorTopsqlMysql)

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](t.mysqlReceiver("metrics"), countConn, s2mConn),
		Processors: common.NewTranslatorMap[component.Config, component.ID](t.scopeTransform(), transformprocessor.NewTranslatorWithName(common.DbiTransformResource+"_mysql_"+idx, transformprocessor.WithMetricStatements(t.resourceStatements())), transformprocessor.NewTranslatorWithName(common.DbiTransformFixStartTimeMysql)),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](fwd),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwd, countConn, s2mConn),
	}, nil
}

func (t *dbiMysqlTranslator) translateLogToMetrics() (*common.ComponentTranslators, error) {
	countConn := count.NewTranslator(common.DbiConnectorDbloadMysql)
	s2mConn := signaltometrics.NewTranslator(common.DbiConnectorTopsqlMysql)

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](t.mysqlReceiver("metrics")),
		Processors: common.NewTranslatorMap[component.Config, component.ID](t.excludeMonitorFilter()),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](countConn, s2mConn),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](countConn, s2mConn),
	}, nil
}

func (t *dbiMysqlTranslator) translateRawEvents() (*common.ComponentTranslators, error) {
	idx := strconv.Itoa(t.instanceIndex)
	fwd := forward.NewTranslator(common.OpenTelemetryKey)

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](t.mysqlReceiver("events")),
		Processors: common.NewTranslatorMap[component.Config, component.ID](t.excludeMonitorFilter(), t.scopeTransform(), resourcedetection.NewTranslator(resourcedetection.WithName(common.OpenTelemetryKey)), transformprocessor.NewTranslatorWithName(common.DbiTransformResource+"_mysql_"+idx, transformprocessor.WithMetricStatements(t.resourceStatements()), transformprocessor.WithLogStatements(t.resourceStatements())), transformprocessor.NewTranslatorWithName(common.DbiTransformLogs+"_mysql_raw-events_"+idx, transformprocessor.WithLogStatements(t.logStatements("raw-events")))),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](fwd),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwd),
	}, nil
}

func (t *dbiMysqlTranslator) translateServerLogs() (*common.ComponentTranslators, error) {
	idx := strconv.Itoa(t.instanceIndex)
	fwd := forward.NewTranslator(common.OpenTelemetryKey)

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](filelog.NewTranslator(filelog.WithNamePrefix("mysql"), filelog.WithIndex(t.instanceIndex), filelog.WithFilePath(t.cfg.logFilePath))),
		Processors: common.NewTranslatorMap[component.Config, component.ID](t.scopeTransform(), resourcedetection.NewTranslator(resourcedetection.WithName(common.OpenTelemetryKey)), transformprocessor.NewTranslatorWithName(common.DbiTransformResource+"_mysql_"+idx, transformprocessor.WithMetricStatements(t.resourceStatements()), transformprocessor.WithLogStatements(t.resourceStatements())), transformprocessor.NewTranslatorWithName(common.DbiTransformLogs+"_mysql_server-logs_"+idx, transformprocessor.WithLogStatements(t.logStatements("server-logs")))),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](fwd),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwd),
	}, nil
}

func (t *dbiMysqlTranslator) mysqlReceiver(name string, extraOpts ...mysql.Option) common.ComponentTranslator {
	opts := []mysql.Option{
		mysql.WithName(name),
		mysql.WithIndex(t.instanceIndex),
		mysql.WithEndpoint(t.cfg.endpoint),
		mysql.WithUsername(t.cfg.username),
		mysql.WithPassfile(t.cfg.passfile),
		mysql.WithCAFile(t.cfg.caFile),
		mysql.WithIsLocalhost(t.cfg.isLocalhost),
	}
	opts = append(opts, extraOpts...)
	return mysql.NewTranslator(opts...)
}

func (t *dbiMysqlTranslator) excludeMonitorFilter() common.ComponentTranslator {
	idx := strconv.Itoa(t.instanceIndex)
	condition := fmt.Sprintf(`attributes["user.name"] == "%s"`, t.cfg.username)
	return filterprocessor.NewTranslatorWithLogCondition(common.DbiFilterExcludeMonitor+"_mysql_"+idx, condition)
}

func (t *dbiMysqlTranslator) scopeTransform() common.ComponentTranslator {
	idx := strconv.Itoa(t.instanceIndex)
	return transformprocessor.NewTranslatorWithName("dbi_scope_mysql_"+idx,
		transformprocessor.WithErrorMode("ignore"),
		transformprocessor.WithScopeStatements([]string{
			`set(attributes["cloudwatch.source"], "cloudwatch-agent")`,
			`set(attributes["cloudwatch.solution"], "otel-database-insights")`,
		}),
	)
}

func (t *dbiMysqlTranslator) resourceStatements() []string {
	return []string{
		`set(resource.attributes["db.system.name"], "mysql")`,
		fmt.Sprintf(`set(resource.attributes["db.instance.name"], "%s")`, t.cfg.instanceName),
	}
}

func (t *dbiMysqlTranslator) logStatements(destination string) []string {
	return []string{
		fmt.Sprintf(`set(resource.attributes["aws.log.group.name"], "/aws/self-managed-database-insights/mysql/%s")`, destination),
		fmt.Sprintf(`set(resource.attributes["aws.log.stream.name"], Concat([resource.attributes["host.id"], "%s"], "/"))`, t.cfg.instanceName),
	}
}
