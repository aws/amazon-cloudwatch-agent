// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package sqlserver

import (
	"strconv"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type Option func(*translator)

type translator struct {
	factory          receiver.Factory
	name             string
	endpoint         string
	username         string
	passfile         string
	index            int
	topQueryInterval time.Duration
}

func WithName(name string) Option   { return func(t *translator) { t.name = name } }
func WithEndpoint(ep string) Option { return func(t *translator) { t.endpoint = ep } }
func WithUsername(u string) Option  { return func(t *translator) { t.username = u } }
func WithPassfile(p string) Option  { return func(t *translator) { t.passfile = p } }
func WithIndex(i int) Option        { return func(t *translator) { t.index = i } }
func WithTopQueryInterval(d time.Duration) Option {
	return func(t *translator) { t.topQueryInterval = d }
}

func NewTranslator(opts ...Option) common.ComponentTranslator {
	t := &translator{
		factory:          sqlserverreceiver.NewFactory(),
		name:             "metrics",
		topQueryInterval: 60 * time.Second,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.MustNewIDWithName("sqlserver", t.name+"_"+strconv.Itoa(t.index))
}

// splitEndpoint splits "host:port" into server + port, defaulting to 1433.
func splitEndpoint(endpoint string) (string, uint) {
	host, port := endpoint, uint(1433)
	if i := strings.LastIndex(endpoint, ":"); i >= 0 {
		host = endpoint[:i]
		if p, err := strconv.ParseUint(endpoint[i+1:], 10, 32); err == nil {
			port = uint(p)
		}
	}
	return host, port
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*sqlserverreceiver.Config)

	cfg.Server, cfg.Port = splitEndpoint(t.endpoint)
	cfg.Username = t.username
	cfg.Passfile = t.passfile

	// Enable metrics disabled-by-default in the receiver but required by DBI.
	// These metrics align with RDS SQL Server monitoring best practices.
	
	// Performance & Resource Metrics (RDS equivalents)
	cfg.MetricsBuilderConfig.Metrics.SqlserverProcessesBlocked.Enabled = true          // RDS: BlockedTransactions
	cfg.MetricsBuilderConfig.Metrics.SqlserverDeadlockRate.Enabled = true             // RDS: Deadlocks/sec
	cfg.MetricsBuilderConfig.Metrics.SqlserverMemoryGrantsPendingCount.Enabled = true // RDS: Memory Grants Pending
	cfg.MetricsBuilderConfig.Metrics.SqlserverMemoryUsage.Enabled = true              // RDS: FreeableMemory (inverse)
	
	// Page & Buffer Pool Metrics (RDS equivalents)
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageLookupRate.Enabled = true          // RDS: Page Lookups/sec
	cfg.MetricsBuilderConfig.Metrics.SqlserverForwardedRecordsRate.Enabled = true    // Performance tuning metric
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageBufferCacheFreeListStallsRate.Enabled = true // Memory pressure indicator
	
	// Wait Statistics (RDS Performance Insights equivalents)
	cfg.MetricsBuilderConfig.Metrics.SqlserverLatchWaitRate.Enabled = true   // Concurrency metric
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockTimeoutRate.Enabled = true // Lock contention
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockWaitCount.Enabled = true   // Lock contention
	
	// Transaction & Log Metrics (RDS equivalents)
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionActiveCount.Enabled = true         // RDS: Active Transactions
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionLogFlushRate.Enabled = true        // RDS: Log Flushes/sec
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionLogFlushWaitRate.Enabled = true    // Log IO bottleneck
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionLogUsage.Enabled = true            // RDS: TransactionLogUsage
	
	// Database I/O Metrics (RDS: ReadIOPS, WriteIOPS, ReadThroughput, WriteThroughput equivalents)
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseIo.Enabled = true       // Per-database I/O
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseLatency.Enabled = true  // Per-database latency
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseOperations.Enabled = true // I/O operations
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseIoStallQueued.Enabled = true // Resource Governor I/O queue wait time
	
	// Connection & Workload Metrics (RDS equivalents)
	cfg.MetricsBuilderConfig.Metrics.SqlserverLoginRate.Enabled = true  // RDS: LoginFailures (partial)
	cfg.MetricsBuilderConfig.Metrics.SqlserverLogoutRate.Enabled = true // Connection churn
	cfg.MetricsBuilderConfig.Metrics.SqlserverSessionCount.Enabled = true // Active sessions by state
	
	// Index & Query Performance Metrics
	cfg.MetricsBuilderConfig.Metrics.SqlserverIndexSearchRate.Enabled = true        // Index usage
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseFullScanRate.Enabled = true   // Table scan indicator (performance issue)
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseExecutionErrors.Enabled = true // RDS: Errors/sec
	
	// Optional: TempDB Metrics (useful for workload analysis)
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseTempdbSpace.Enabled = true            // TempDB space usage
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseTempdbVersionStoreSize.Enabled = true // Version store size

	cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true

	cfg.QuerySample.MaxRowsPerQuery = 500
	cfg.TopQueryCollection.CollectionInterval = t.topQueryInterval
	cfg.TopQueryCollection.TopQueryCount = 200
	cfg.TopQueryCollection.MaxQuerySampleCount = 5000
	
	// Query Plan Caching: Controls caching behavior for SQL Server XML ShowPlan data
	// Plans are automatically compressed (always-on) to fit within CloudWatch Logs 1MB limit
	cfg.TopQueryCollection.QueryPlanCacheSize = 1000       // Cache up to 1000 plans
	cfg.TopQueryCollection.QueryPlanCacheTTL = time.Hour   // Keep plans for 1 hour
	cfg.TopQueryCollection.MaxQueryPlanSize = 921600       // Max 900KB after compression (safety limit)

	return cfg, nil
}
