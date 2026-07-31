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

	translatorconfig "github.com/aws/amazon-cloudwatch-agent/translator/config"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator/context"
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
	
	// Performance & Resource Metrics 
	cfg.MetricsBuilderConfig.Metrics.SqlserverProcessesBlocked.Enabled = true        
	cfg.MetricsBuilderConfig.Metrics.SqlserverDeadlockRate.Enabled = true             
	cfg.MetricsBuilderConfig.Metrics.SqlserverMemoryGrantsPendingCount.Enabled = true 
	cfg.MetricsBuilderConfig.Metrics.SqlserverMemoryUsage.Enabled = true              
	
	// Page & Buffer Pool Metrics 
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageLookupRate.Enabled = true          
	cfg.MetricsBuilderConfig.Metrics.SqlserverForwardedRecordsRate.Enabled = true    
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageBufferCacheFreeListStallsRate.Enabled = true 
	
	// Wait Statistics
	cfg.MetricsBuilderConfig.Metrics.SqlserverLatchWaitRate.Enabled = true   
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockTimeoutRate.Enabled = true 
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockWaitCount.Enabled = true 
	
	// Windows-only metrics: These use Windows Performance Monitor counters (via recorders.go)
	// that are unavailable on Linux. The SQL query fetches the data from sys.dm_os_performance_counters
	// but scraper.go does not process them on the direct-connect (Linux) path.
	isWindows := translatorcontext.CurrentContext().Os() == translatorconfig.OS_TYPE_WINDOWS
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockWaitTimeAvg.Enabled = isWindows
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageCheckpointFlushRate.Enabled = isWindows
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageLazyWriteRate.Enabled = isWindows
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageOperationRate.Enabled = isWindows
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageSplitRate.Enabled = isWindows
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionRate.Enabled = isWindows
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionWriteRate.Enabled = isWindows
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionLogFlushDataRate.Enabled = isWindows
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionLogFlushRate.Enabled = isWindows
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionLogFlushWaitRate.Enabled = isWindows
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionLogGrowthCount.Enabled = isWindows
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionLogShrinkCount.Enabled = isWindows
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionLogUsage.Enabled = isWindows         
	
	// Database I/O Metrics 
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseIo.Enabled = true     
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseLatency.Enabled = true  
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseOperations.Enabled = true 
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseIoStallQueued.Enabled = true 
	
	// Connection & Workload Metrics 
	cfg.MetricsBuilderConfig.Metrics.SqlserverLoginRate.Enabled = true  
	cfg.MetricsBuilderConfig.Metrics.SqlserverLogoutRate.Enabled = true 
	cfg.MetricsBuilderConfig.Metrics.SqlserverSessionCount.Enabled = true 
	// Index & Query Performance Metrics
	cfg.MetricsBuilderConfig.Metrics.SqlserverIndexSearchRate.Enabled = true        
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseFullScanRate.Enabled = true  
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseExecutionErrors.Enabled = true 
	
	// Optional: TempDB Metrics (useful for workload analysis)
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseTempdbSpace.Enabled = true            
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseTempdbVersionStoreSize.Enabled = true 

	cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true

	cfg.QuerySample.MaxRowsPerQuery = 5000
	cfg.TopQueryCollection.CollectionInterval = t.topQueryInterval
	cfg.TopQueryCollection.TopQueryCount = 200
	cfg.TopQueryCollection.MaxQuerySampleCount = 1000
	
	// Query Plan Caching: Controls caching behavior for SQL Server XML ShowPlan data
	// Plans are automatically compressed (always-on) to fit within CloudWatch Logs 1MB limit
	cfg.TopQueryCollection.QueryPlanCacheSize = 1000       // Cache up to 1000 plans
	cfg.TopQueryCollection.QueryPlanCacheTTL = time.Hour   // Keep plans for 1 hour
	cfg.TopQueryCollection.MaxQueryPlanSize = 921600       // Max 900KB after compression (safety limit)

	return cfg, nil
}
