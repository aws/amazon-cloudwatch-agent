// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltometrics

import (
	"testing"

	signaltometricsconfig "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslatorID(t *testing.T) {
	tr := NewTranslator(common.DbiConnectorTopsql+"_"+common.PostgreSQLKey, common.PostgreSQLKey)
	assert.Equal(t, "signaltometrics/dbi_topsql_postgresql", tr.ID().String())
}

func TestTranslateTopsql(t *testing.T) {
	tr := NewTranslator(common.DbiConnectorTopsql+"_"+common.PostgreSQLKey, common.PostgreSQLKey)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	stmCfg := cfg.(*signaltometricsconfig.Config)
	assert.Len(t, stmCfg.Logs, 8)
	assert.Equal(t, "postgresql.calls", stmCfg.Logs[0].Name)
	assert.Equal(t, "postgresql.total_exec_time", stmCfg.Logs[1].Name)
	assert.Equal(t, "postgresql.total_plan_time", stmCfg.Logs[2].Name)
	assert.Equal(t, "postgresql.rows", stmCfg.Logs[3].Name)
	assert.Equal(t, "postgresql.shared_blks_hit", stmCfg.Logs[4].Name)
	assert.Equal(t, "postgresql.shared_blks_read", stmCfg.Logs[5].Name)
	assert.Equal(t, "postgresql.local_blks_hit", stmCfg.Logs[6].Name)
	assert.Equal(t, "postgresql.local_blks_read", stmCfg.Logs[7].Name)
}

func TestTranslateUnsupported(t *testing.T) {
	tr := NewTranslator("dbi_topsql_unknown", "unsupported")
	_, err := tr.Translate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported signaltometrics connector engine")
}

func TestTranslatorID_MySQL(t *testing.T) {
	tr := NewTranslator(common.DbiConnectorTopsql+"_"+common.MySQLKey, common.MySQLKey)
	assert.Equal(t, "signaltometrics/dbi_topsql_mysql", tr.ID().String())
}

func TestTranslateTopsqlMysql(t *testing.T) {
	tr := NewTranslator(common.DbiConnectorTopsql+"_"+common.MySQLKey, common.MySQLKey)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	stmCfg := cfg.(*signaltometricsconfig.Config)
	assert.Len(t, stmCfg.Logs, 18)
	assert.Equal(t, "mysql.count_star", stmCfg.Logs[0].Name)
	assert.Equal(t, "mysql.sum_timer_wait", stmCfg.Logs[1].Name)
	assert.Equal(t, "mysql.sum_lock_time", stmCfg.Logs[2].Name)
	assert.Equal(t, "mysql.sum_rows_sent", stmCfg.Logs[3].Name)
	assert.Equal(t, "mysql.sum_rows_examined", stmCfg.Logs[4].Name)
	assert.Equal(t, "mysql.sum_errors", stmCfg.Logs[5].Name)
	assert.Equal(t, "mysql.sum_sort_rows", stmCfg.Logs[6].Name)
	assert.Equal(t, "mysql.sum_created_tmp_tables", stmCfg.Logs[7].Name)
	assert.Equal(t, "mysql.sum_created_tmp_disk_tables", stmCfg.Logs[8].Name)
	assert.Equal(t, "mysql.sum_no_index_used", stmCfg.Logs[9].Name)
	assert.Equal(t, "mysql.sum_select_full_join", stmCfg.Logs[10].Name)
	assert.Equal(t, "mysql.sum_sort_scan", stmCfg.Logs[11].Name)
	assert.Equal(t, "mysql.sum_no_good_index_used", stmCfg.Logs[12].Name)
	assert.Equal(t, "mysql.sum_select_scan", stmCfg.Logs[13].Name)
	assert.Equal(t, "mysql.sum_rows_affected", stmCfg.Logs[14].Name)
	assert.Equal(t, "mysql.sum_select_range_check", stmCfg.Logs[15].Name)
	assert.Equal(t, "mysql.sum_sort_merge_passes", stmCfg.Logs[16].Name)
	assert.Equal(t, "mysql.sum_sort_range", stmCfg.Logs[17].Name)
}
