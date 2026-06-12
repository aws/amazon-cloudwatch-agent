// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package databaseinsights

import (
	"fmt"
	"strconv"
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
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](t.pgReceiver("metrics"), countConn, s2mConn),
		Processors: common.NewTranslatorMap[component.Config, component.ID](transformprocessor.NewTranslatorWithName(common.DbiTransformResource+"_"+idx, transformprocessor.WithMetricStatements(t.resourceStatements())), transformprocessor.NewTranslatorWithName(common.DbiTransformFixStartTime)),
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
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](t.pgReceiver("events", postgresql.WithQuerySampleInterval(60*time.Second))),
		Processors: common.NewTranslatorMap[component.Config, component.ID](t.excludeMonitorFilter(), resourcedetection.NewTranslator(resourcedetection.WithName(common.OpenTelemetryKey)), transformprocessor.NewTranslatorWithName(common.DbiTransformResource+"_"+idx, transformprocessor.WithMetricStatements(t.resourceStatements()), transformprocessor.WithLogStatements(t.resourceStatements())), transformprocessor.NewTranslatorWithName(common.DbiTransformLogs+"_raw-events_"+idx, transformprocessor.WithLogStatements(t.logStatements("raw-events")))),
		Exporters:  common.NewTranslatorMap[component.Config, component.ID](fwd),
		Extensions: common.NewTranslatorMap[component.Config, component.ID](),
		Connectors: common.NewTranslatorMap[component.Config, component.ID](fwd),
	}, nil
}

func (t *dbiTranslator) translateServerLogs() (*common.ComponentTranslators, error) {
	idx := strconv.Itoa(t.instanceIndex)
	fwd := forward.NewTranslator(common.OpenTelemetryKey)

	return &common.ComponentTranslators{
		Receivers:  common.NewTranslatorMap[component.Config, component.ID](filelog.NewTranslator(filelog.WithNamePrefix("postgresql"), filelog.WithIndex(t.instanceIndex), filelog.WithFilePath(t.cfg.logFilePath))),
		Processors: common.NewTranslatorMap[component.Config, component.ID](resourcedetection.NewTranslator(resourcedetection.WithName(common.OpenTelemetryKey)), transformprocessor.NewTranslatorWithName(common.DbiTransformResource+"_"+idx, transformprocessor.WithMetricStatements(t.resourceStatements()), transformprocessor.WithLogStatements(t.resourceStatements())), transformprocessor.NewTranslatorWithName(common.DbiTransformLogs+"_server-logs_"+idx, transformprocessor.WithLogStatements(t.logStatements("server-logs")))),
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
	return filterprocessor.NewTranslatorWithLogCondition(common.DbiFilterExcludeMonitor+"_"+idx, condition)
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
