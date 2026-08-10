// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package postgresql

import (
	"reflect"
	"strconv"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

// metricScope selects which subset of postgresql metrics a receiver instance emits.
// It exists so DBI can run two postgresql receiver instances against the same database:
// a server-metrics instance at the default (10s) interval and a per-resource
// (table/index) instance at a slower (60s) interval, to cap per-resource ingestion
// volume without slowing the server metrics.
type metricScope int

const (
	// metricScopeAll keeps the receiver's default metric set (current behavior).
	metricScopeAll metricScope = iota
	// metricScopeServerOnly disables the per-resource (table/index) metrics, leaving
	// the server/per-database metrics enabled.
	metricScopeServerOnly
	// metricScopePerResourceOnly enables ONLY the per-resource (table/index) metrics
	// and disables everything else.
	metricScopePerResourceOnly
	// metricScopeNone disables every metric. Used by receiver instances that are wired
	// into a logs pipeline only, so their metrics are never collected and leaving them
	// enabled would misrepresent what the instance emits.
	metricScopeNone
)

type Option func(*translator)

type translator struct {
	factory             receiver.Factory
	name                string
	endpoint            string
	username            string
	passfile            string
	caFile              string
	isLocalhost         bool
	index               int
	querySampleInterval time.Duration
	collectionInterval  time.Duration
	metricScope         metricScope
	eventsDisabled      bool
}

func WithName(name string) Option   { return func(t *translator) { t.name = name } }
func WithEndpoint(ep string) Option { return func(t *translator) { t.endpoint = ep } }
func WithUsername(u string) Option  { return func(t *translator) { t.username = u } }
func WithPassfile(p string) Option  { return func(t *translator) { t.passfile = p } }
func WithCAFile(ca string) Option   { return func(t *translator) { t.caFile = ca } }
func WithIsLocalhost(b bool) Option { return func(t *translator) { t.isLocalhost = b } }
func WithIndex(i int) Option        { return func(t *translator) { t.index = i } }
func WithQuerySampleInterval(d time.Duration) Option {
	return func(t *translator) { t.querySampleInterval = d }
}

// WithCollectionInterval overrides the receiver's top-level scrape interval. When unset
// (zero), the receiver inherits the upstream factory default (10s).
func WithCollectionInterval(d time.Duration) Option {
	return func(t *translator) { t.collectionInterval = d }
}

// WithServerMetricsOnly disables the per-resource (table/index) metrics so this
// instance emits only server/per-database metrics.
func WithServerMetricsOnly() Option {
	return func(t *translator) { t.metricScope = metricScopeServerOnly }
}

// WithPerResourceMetricsOnly enables only the per-resource (table/index) metrics and
// disables everything else, for the dedicated slower-interval instance.
func WithPerResourceMetricsOnly() Option {
	return func(t *translator) { t.metricScope = metricScopePerResourceOnly }
}

// WithMetricsDisabled disables every metric on this instance. Used for the events
// receiver, which is wired into a logs pipeline only: no metrics pipeline consumes it,
// so an enabled metric there is never collected and only misleads anyone reading the
// generated config.
func WithMetricsDisabled() Option {
	return func(t *translator) { t.metricScope = metricScopeNone }
}

// WithEventsDisabled turns off the query-sample and top-query event streams. Used for
// the metrics-only per-resource instance so it never duplicates the events (DBLoad /
// TopSQL) produced by the primary server instance.
func WithEventsDisabled() Option {
	return func(t *translator) { t.eventsDisabled = true }
}

func NewTranslator(opts ...Option) common.ComponentTranslator {
	t := &translator{
		factory:             postgresqlreceiver.NewFactory(),
		name:                "metrics",
		querySampleInterval: time.Second,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *translator) ID() component.ID {
	return component.MustNewIDWithName("postgresql", t.name+"_"+strconv.Itoa(t.index))
}

func (t *translator) Translate(_ *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*postgresqlreceiver.Config)

	cfg.Endpoint = t.endpoint
	cfg.Username = t.username
	cfg.Passfile = t.passfile
	cfg.Transport = "tcp"

	if t.isLocalhost {
		cfg.Insecure = true
		cfg.InsecureSkipVerify = true
	} else {
		cfg.CAFile = t.caFile
		cfg.InsecureSkipVerify = false
	}

	cfg.Metrics.PostgresqlFunctionCalls.Enabled = false
	cfg.Metrics.PostgresqlTupDeleted.Enabled = false
	cfg.Metrics.PostgresqlTupFetched.Enabled = false
	cfg.Metrics.PostgresqlTupInserted.Enabled = false
	cfg.Metrics.PostgresqlTupReturned.Enabled = false
	cfg.Metrics.PostgresqlTupUpdated.Enabled = false
	cfg.Metrics.PostgresqlWalDelay.Enabled = false

	// Split the metric set when requested so DBI can collect per-resource (table/index)
	// metrics on a separate, slower receiver instance than the server metrics. Both
	// modes only ever DISABLE metrics, so the split preserves the exact set of metrics
	// emitted today (partitioned by cadence) rather than adding or dropping any.
	switch t.metricScope {
	case metricScopeAll:
		// Keep the receiver's default metric set: no split requested.
	case metricScopeServerOnly:
		// Move the per-resource (table/index) metrics off to the dedicated instance.
		forEachMetric(cfg, func(name string, enabled *bool) {
			if perResourceMetricNames[name] {
				*enabled = false
			}
		})
	case metricScopePerResourceOnly:
		// Keep only the per-resource metrics; disable everything else. Per-resource
		// metrics are left at their existing enabled state, so a metric that was off
		// (e.g. sequential_scans) is not newly introduced by the split.
		forEachMetric(cfg, func(name string, enabled *bool) {
			if !perResourceMetricNames[name] {
				*enabled = false
			}
		})
	case metricScopeNone:
		// Logs-only instance: collect no metrics at all.
		forEachMetric(cfg, func(_ string, enabled *bool) {
			*enabled = false
		})
	}

	if t.eventsDisabled {
		cfg.Events.DbServerQuerySample.Enabled = false
		cfg.Events.DbServerTopQuery.Enabled = false
	} else {
		cfg.Events.DbServerQuerySample.Enabled = true
		cfg.Events.DbServerTopQuery.Enabled = true
	}

	if t.collectionInterval > 0 {
		cfg.ControllerConfig.CollectionInterval = t.collectionInterval
	}

	cfg.Enabled = true
	cfg.QuerySampleCollection.CollectionInterval = t.querySampleInterval
	cfg.QuerySampleCollection.MaxRowsPerQuery = 500

	cfg.TopQueryCollection.CollectionInterval = 60 * time.Second
	cfg.TopNQuery = 200
	cfg.TopQueryCollection.MaxRowsPerQuery = 5000
	cfg.MaxExplainEachInterval = 1000

	return cfg, nil
}

// perResourceMetricNames is the set of per-resource (table/index) metrics, keyed by
// mapstructure name. These are emitted by the receiver's collectTables / collectIndexes
// with a postgresql.table.name / postgresql.index.name resource attribute.
// postgresql.table.count is intentionally excluded: it is a per-database server metric
// recorded alongside the other server metrics.
var perResourceMetricNames = map[string]bool{
	"postgresql.rows":               true,
	"postgresql.operations":         true,
	"postgresql.table.size":         true,
	"postgresql.table.vacuum.count": true,
	"postgresql.sequential_scans":   true,
	"postgresql.blocks_read":        true,
	"postgresql.index.scans":        true,
	"postgresql.index.size":         true,
}

// forEachMetric invokes fn for every metric in the receiver's MetricsConfig, passing the
// metric's mapstructure name and a pointer to its Enabled flag. It uses reflection so
// metrics added upstream in the future are handled without changes here.
func forEachMetric(cfg *postgresqlreceiver.Config, fn func(name string, enabled *bool)) {
	v := reflect.ValueOf(&cfg.Metrics).Elem()
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		ef := v.Field(i).FieldByName("Enabled")
		if !ef.IsValid() || ef.Kind() != reflect.Bool || !ef.CanAddr() {
			continue
		}
		fn(typ.Field(i).Tag.Get("mapstructure"), ef.Addr().Interface().(*bool))
	}
}
