// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

// DROP-IN LOCATION (amazon-cloudwatch-agent repo):
//   translator/translate/otel/receiver/sqlserver/translator_test.go
//
// This is a TDD spec: it defines the exact behavior the SQL Server receiver
// translator must produce. Implement translator.go in the same package so this
// test passes. It mirrors receiver/mysql/translator_test.go, adapted for the
// sqlserverreceiver Config (Server+Port instead of Endpoint; QuerySample instead
// of QuerySampleCollection).

package sqlserver

import (
	"runtime"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslator_ID(t *testing.T) {
	tr := NewTranslator(WithName("metrics"), WithIndex(0))
	assert.Equal(t, "sqlserver/metrics_0", tr.ID().String())

	tr = NewTranslator(WithName("events"), WithIndex(1))
	assert.Equal(t, "sqlserver/events_1", tr.ID().String())
}

func TestTranslator_Translate_Localhost(t *testing.T) {
	tr := NewTranslator(
		WithEndpoint("localhost:1433"),
		WithUsername("cw_monitor"),
		WithPassfile("/opt/databaseinsights/.sqlserver_credentials"),
		WithIndex(0),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	c := cfg.(*sqlserverreceiver.Config)

	// Connection: endpoint "host:port" is split into Server + Port.
	assert.Equal(t, "localhost", c.Server)
	assert.Equal(t, uint(1433), c.Port)
	assert.Equal(t, "cw_monitor", c.Username)
	assert.Equal(t, "/opt/databaseinsights/.sqlserver_credentials", c.Passfile)
	assert.Empty(t, string(c.Password)) // passfile is used, never an inline password
	assert.Empty(t, c.DataSource)       // never combine datasource with discrete fields

	// DBI requires the two log events.
	assert.True(t, c.LogsBuilderConfig.Events.DbServerQuerySample.Enabled)
	assert.True(t, c.LogsBuilderConfig.Events.DbServerTopQuery.Enabled)

	// DB Load / Top SQL tuning (match the mysql translator's values).
	assert.Equal(t, uint64(5000), c.QuerySample.MaxRowsPerQuery)
	assert.Equal(t, uint(200), c.TopQueryCollection.TopQueryCount)
	assert.Equal(t, uint(1000), c.TopQueryCollection.MaxQuerySampleCount)
	assert.Equal(t, 60*time.Second, c.TopQueryCollection.CollectionInterval)

	// Query Plan Caching: verify all 3 fields (compression is always-on in receiver).
	assert.Equal(t, 1000, c.TopQueryCollection.QueryPlanCacheSize)
	assert.Equal(t, time.Hour, c.TopQueryCollection.QueryPlanCacheTTL)
	assert.Equal(t, 921600, c.TopQueryCollection.MaxQueryPlanSize) // 900KB default

	// Metrics that are disabled-by-default in the receiver but required for DBI
	m := c.MetricsBuilderConfig.Metrics

	// Performance & Resource Metrics
	assert.True(t, m.SqlserverProcessesBlocked.Enabled)
	assert.True(t, m.SqlserverDeadlockRate.Enabled)
	assert.True(t, m.SqlserverMemoryGrantsPendingCount.Enabled)
	assert.True(t, m.SqlserverMemoryUsage.Enabled)

	// Page & Buffer Pool Metrics
	assert.True(t, m.SqlserverPageLookupRate.Enabled)
	assert.True(t, m.SqlserverForwardedRecordsRate.Enabled)
	assert.True(t, m.SqlserverPageBufferCacheFreeListStallsRate.Enabled)

	// Wait Statistics (RDS Performance Insights equivalents)
	assert.True(t, m.SqlserverLatchWaitRate.Enabled)
	assert.True(t, m.SqlserverLockTimeoutRate.Enabled)
	assert.True(t, m.SqlserverLockWaitCount.Enabled)

	// Windows-only metrics: use Windows Performance Monitor counters unavailable on Linux
	if runtime.GOOS == "windows" {
		assert.True(t, m.SqlserverLockWaitTimeAvg.Enabled)
		assert.True(t, m.SqlserverPageCheckpointFlushRate.Enabled)
		assert.True(t, m.SqlserverPageLazyWriteRate.Enabled)
		assert.True(t, m.SqlserverPageOperationRate.Enabled)
		assert.True(t, m.SqlserverPageSplitRate.Enabled)
		assert.True(t, m.SqlserverTransactionRate.Enabled)
		assert.True(t, m.SqlserverTransactionWriteRate.Enabled)
		assert.True(t, m.SqlserverTransactionLogFlushDataRate.Enabled)
		assert.True(t, m.SqlserverTransactionLogFlushRate.Enabled)
		assert.True(t, m.SqlserverTransactionLogFlushWaitRate.Enabled)
		assert.True(t, m.SqlserverTransactionLogGrowthCount.Enabled)
		assert.True(t, m.SqlserverTransactionLogShrinkCount.Enabled)
		assert.True(t, m.SqlserverTransactionLogUsage.Enabled)
	} else {
		assert.False(t, m.SqlserverLockWaitTimeAvg.Enabled)
		assert.False(t, m.SqlserverPageCheckpointFlushRate.Enabled)
		assert.False(t, m.SqlserverPageLazyWriteRate.Enabled)
		assert.False(t, m.SqlserverPageOperationRate.Enabled)
		assert.False(t, m.SqlserverPageSplitRate.Enabled)
		assert.False(t, m.SqlserverTransactionRate.Enabled)
		assert.False(t, m.SqlserverTransactionWriteRate.Enabled)
		assert.False(t, m.SqlserverTransactionLogFlushDataRate.Enabled)
		assert.False(t, m.SqlserverTransactionLogFlushRate.Enabled)
		assert.False(t, m.SqlserverTransactionLogFlushWaitRate.Enabled)
		assert.False(t, m.SqlserverTransactionLogGrowthCount.Enabled)
		assert.False(t, m.SqlserverTransactionLogShrinkCount.Enabled)
		assert.False(t, m.SqlserverTransactionLogUsage.Enabled)
	}

	// Database I/O Metrics
	assert.True(t, m.SqlserverDatabaseIo.Enabled)
	assert.True(t, m.SqlserverDatabaseLatency.Enabled)
	assert.True(t, m.SqlserverDatabaseOperations.Enabled)
	assert.True(t, m.SqlserverDatabaseIoStallQueued.Enabled)

	// Connection & Workload Metrics
	assert.True(t, m.SqlserverLoginRate.Enabled)
	assert.True(t, m.SqlserverLogoutRate.Enabled)
	assert.True(t, m.SqlserverSessionCount.Enabled)

	// Index & Query Performance Metrics
	assert.True(t, m.SqlserverIndexSearchRate.Enabled)
	assert.True(t, m.SqlserverDatabaseFullScanRate.Enabled)
	assert.True(t, m.SqlserverDatabaseExecutionErrors.Enabled)

	// Optional: TempDB Metrics (useful for workload analysis)
	assert.True(t, m.SqlserverDatabaseTempdbSpace.Enabled)
	assert.True(t, m.SqlserverDatabaseTempdbVersionStoreSize.Enabled)
}

func TestTranslator_Translate_DefaultPort(t *testing.T) {
	// An endpoint with no port must default to 1433.
	tr := NewTranslator(WithEndpoint("localhost"), WithUsername("cw_monitor"))
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	c := cfg.(*sqlserverreceiver.Config)
	assert.Equal(t, "localhost", c.Server)
	assert.Equal(t, uint(1433), c.Port)
}

func TestTranslator_Translate_CustomInterval(t *testing.T) {
	tr := NewTranslator(
		WithEndpoint("localhost:1433"),
		WithUsername("cw_monitor"),
		WithTopQueryInterval(30*time.Second),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, cfg.(*sqlserverreceiver.Config).TopQueryCollection.CollectionInterval)
}

func TestTranslator_QueryPlanCachingDefaults(t *testing.T) {
	// Verify that query plan caching is configured with correct defaults.
	// These 3 fields control resource usage and safety limits.
	// NOTE: Compression is always-on in the receiver (not configurable).
	tr := NewTranslator(
		WithEndpoint("localhost:1433"),
		WithUsername("cw_monitor"),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	c := cfg.(*sqlserverreceiver.Config)

	// Verify the 3 query plan caching fields
	assert.Equal(t, 1000, c.TopQueryCollection.QueryPlanCacheSize,
		"Default cache size should be 1000 plans")
	assert.Equal(t, time.Hour, c.TopQueryCollection.QueryPlanCacheTTL,
		"Default cache TTL should be 1 hour")
	assert.Equal(t, 921600, c.TopQueryCollection.MaxQueryPlanSize,
		"Default max plan size should be 900KB (921600 bytes) after compression")
}
