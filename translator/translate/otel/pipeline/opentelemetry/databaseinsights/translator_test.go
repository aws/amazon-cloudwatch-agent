// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package databaseinsights

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestDbiTranslatorID(t *testing.T) {
	tests := []struct {
		name         string
		engine       string
		pipelineType dbiPipelineType
		index        int
		want         string
	}{
		{"PG_Metrics_0", common.PostgreSQLKey, dbiMetrics, 0, "metrics/dbi_postgresql_0"},
		{"PG_Metrics_1", common.PostgreSQLKey, dbiMetrics, 1, "metrics/dbi_postgresql_1"},
		{"PG_LogToMetrics_0", common.PostgreSQLKey, dbiLogToMetrics, 0, "logs/dbi_postgresql_0"},
		{"PG_RawEvents_0", common.PostgreSQLKey, dbiRawEvents, 0, "logs/dbi_postgresql_rawevents_0"},
		{"PG_ServerLogs_0", common.PostgreSQLKey, dbiServerLogs, 0, "logs/dbi_postgresql_serverlogs_0"},
		{"Mysql_Metrics_0", common.MySQLKey, dbiMetrics, 0, "metrics/dbi_mysql_0"},
		{"Mysql_Metrics_1", common.MySQLKey, dbiMetrics, 1, "metrics/dbi_mysql_1"},
		{"Mysql_LogToMetrics_0", common.MySQLKey, dbiLogToMetrics, 0, "logs/dbi_mysql_0"},
		{"Mysql_RawEvents_0", common.MySQLKey, dbiRawEvents, 0, "logs/dbi_mysql_rawevents_0"},
		{"Mysql_ServerLogs_0", common.MySQLKey, dbiServerLogs, 0, "logs/dbi_mysql_serverlogs_0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := &dbiTranslator{pipelineType: tc.pipelineType, instanceIndex: tc.index, cfg: dbiInstanceConfig{engine: tc.engine}}
			assert.Equal(t, tc.want, tr.ID().String())
		})
	}
}

func TestDbiTranslate(t *testing.T) {
	cfg := dbiInstanceConfig{
		engine:       common.PostgreSQLKey,
		endpoint:     "localhost:5432",
		username:     "cw_monitor",
		passfile:     "/etc/.pgpass",
		instanceName: "my-db",
		logFilePath:  "/var/log/postgresql/postgresql.log",
		isLocalhost:  true,
	}

	tests := []struct {
		name       string
		pipeline   dbiPipelineType
		expectedID string
		nRecv      int
		nProc      int
		nExp       int
		nConn      int
	}{
		{"metrics", dbiMetrics, "metrics/dbi_postgresql_0", 3, 3, 1, 3},
		{"log_to_metrics", dbiLogToMetrics, "logs/dbi_postgresql_0", 1, 1, 2, 2},
		{"raw_events", dbiRawEvents, "logs/dbi_postgresql_rawevents_0", 1, 5, 1, 1},
		{"server_logs", dbiServerLogs, "logs/dbi_postgresql_serverlogs_0", 1, 4, 1, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := &dbiTranslator{pipelineType: tc.pipeline, instanceIndex: 0, cfg: cfg}
			assert.Equal(t, tc.expectedID, tr.ID().String())

			result, err := tr.Translate(nil)
			require.NoError(t, err)
			assert.Equal(t, tc.nRecv, result.Receivers.Len())
			assert.Equal(t, tc.nProc, result.Processors.Len())
			assert.Equal(t, tc.nExp, result.Exporters.Len())
			assert.Equal(t, tc.nConn, result.Connectors.Len())
		})
	}
}

func TestDbiTranslateMetrics_ComponentIDs(t *testing.T) {
	tr := &dbiTranslator{
		pipelineType:  dbiMetrics,
		instanceIndex: 0,
		cfg:           dbiInstanceConfig{engine: common.PostgreSQLKey, instanceName: "mydb"},
	}
	result, err := tr.Translate(nil)
	require.NoError(t, err)

	var receivers, processors, exporters, connectors []string
	result.Receivers.Range(func(c common.Translator[component.Config, component.ID]) {
		receivers = append(receivers, c.ID().String())
	})
	result.Processors.Range(func(c common.Translator[component.Config, component.ID]) {
		processors = append(processors, c.ID().String())
	})
	result.Exporters.Range(func(c common.Translator[component.Config, component.ID]) {
		exporters = append(exporters, c.ID().String())
	})
	result.Connectors.Range(func(c common.Translator[component.Config, component.ID]) {
		connectors = append(connectors, c.ID().String())
	})

	assert.ElementsMatch(t, []string{"postgresql/metrics_0", "count/dbi_dbload_postgresql", "signaltometrics/dbi_topsql_postgresql"}, receivers)
	assert.Equal(t, []string{"transform/dbi_scope_postgresql_0", "transform/dbi_resource_postgresql_0", "transform/dbi_fix_start_time_postgresql"}, processors)
	assert.ElementsMatch(t, []string{"forward/opentelemetry"}, exporters)
	assert.ElementsMatch(t, []string{"forward/opentelemetry", "count/dbi_dbload_postgresql", "signaltometrics/dbi_topsql_postgresql"}, connectors)
}

func TestDbiMysqlTranslate(t *testing.T) {
	cfg := dbiInstanceConfig{
		engine:       common.MySQLKey,
		endpoint:     "localhost:3306",
		username:     "cw_monitor",
		passfile:     "/etc/.mysql_credentials",
		instanceName: "my-db",
		logFilePath:  "/var/log/mysql/mysql.log",
		isLocalhost:  true,
	}

	tests := []struct {
		name       string
		pipeline   dbiPipelineType
		expectedID string
		nRecv      int
		nProc      int
		nExp       int
		nConn      int
	}{
		{"metrics", dbiMetrics, "metrics/dbi_mysql_0", 3, 3, 1, 3},
		{"log_to_metrics", dbiLogToMetrics, "logs/dbi_mysql_0", 1, 1, 2, 2},
		{"raw_events", dbiRawEvents, "logs/dbi_mysql_rawevents_0", 1, 5, 1, 1},
		{"server_logs", dbiServerLogs, "logs/dbi_mysql_serverlogs_0", 1, 4, 1, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := &dbiTranslator{pipelineType: tc.pipeline, instanceIndex: 0, cfg: cfg}
			assert.Equal(t, tc.expectedID, tr.ID().String())

			result, err := tr.Translate(nil)
			require.NoError(t, err)
			assert.Equal(t, tc.nRecv, result.Receivers.Len())
			assert.Equal(t, tc.nProc, result.Processors.Len())
			assert.Equal(t, tc.nExp, result.Exporters.Len())
			assert.Equal(t, tc.nConn, result.Connectors.Len())
		})
	}
}

func TestDbiMysqlTranslateMetrics_ComponentIDs(t *testing.T) {
	tr := &dbiTranslator{
		pipelineType:  dbiMetrics,
		instanceIndex: 0,
		cfg:           dbiInstanceConfig{engine: common.MySQLKey, instanceName: "mydb"},
	}
	result, err := tr.Translate(nil)
	require.NoError(t, err)

	var receivers, processors, exporters, connectors []string
	result.Receivers.Range(func(c common.Translator[component.Config, component.ID]) {
		receivers = append(receivers, c.ID().String())
	})
	result.Processors.Range(func(c common.Translator[component.Config, component.ID]) {
		processors = append(processors, c.ID().String())
	})
	result.Exporters.Range(func(c common.Translator[component.Config, component.ID]) {
		exporters = append(exporters, c.ID().String())
	})
	result.Connectors.Range(func(c common.Translator[component.Config, component.ID]) {
		connectors = append(connectors, c.ID().String())
	})

	assert.ElementsMatch(t, []string{"mysql/metrics_0", "count/dbi_dbload_mysql", "signaltometrics/dbi_topsql_mysql"}, receivers)
	assert.Equal(t, []string{"transform/dbi_scope_mysql_0", "transform/dbi_resource_mysql_0", "transform/dbi_fix_start_time_mysql"}, processors)
	assert.ElementsMatch(t, []string{"forward/opentelemetry"}, exporters)
	assert.ElementsMatch(t, []string{"forward/opentelemetry", "count/dbi_dbload_mysql", "signaltometrics/dbi_topsql_mysql"}, connectors)
}
