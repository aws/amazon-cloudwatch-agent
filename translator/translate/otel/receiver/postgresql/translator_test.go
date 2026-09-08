// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package postgresql

import (
	"reflect"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// perResourceMetricNames (the canonical set) is defined in translator.go and shared by
// these tests.

func baseOpts(extra ...Option) []Option {
	return append([]Option{
		WithEndpoint("localhost:5432"),
		WithUsername("cw_monitor"),
		WithPassfile("/etc/.pgpass"),
		WithIsLocalhost(true),
		WithIndex(0),
	}, extra...)
}

func translateForTest(t *testing.T, opts ...Option) *postgresqlreceiver.Config {
	t.Helper()
	cfg, err := NewTranslator(opts...).Translate(nil)
	require.NoError(t, err)
	return cfg.(*postgresqlreceiver.Config)
}

// enabledMetricNames returns the set of metric names (mapstructure tags) whose Enabled
// flag is true, via reflection over the receiver's MetricsConfig.
func enabledMetricNames(t *testing.T, cfg *postgresqlreceiver.Config) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	v := reflect.ValueOf(cfg.Metrics)
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		ef := v.Field(i).FieldByName("Enabled")
		if ef.IsValid() && ef.Kind() == reflect.Bool && ef.Bool() {
			out[typ.Field(i).Tag.Get("mapstructure")] = true
		}
	}
	return out
}

func TestTranslator_ID(t *testing.T) {
	tr := NewTranslator(WithName("metrics"), WithIndex(0))
	assert.Equal(t, "postgresql/metrics_0", tr.ID().String())

	tr = NewTranslator(WithName("events"), WithIndex(1))
	assert.Equal(t, "postgresql/events_1", tr.ID().String())
}

func TestTranslator_Translate_Defaults(t *testing.T) {
	tr := NewTranslator(
		WithEndpoint("localhost:5432"),
		WithUsername("cw_monitor"),
		WithPassfile("/etc/.pgpass"),
		WithIsLocalhost(true),
		WithIndex(0),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	pgCfg := cfg.(*postgresqlreceiver.Config)

	assert.Equal(t, "localhost:5432", pgCfg.Endpoint)
	assert.Equal(t, "cw_monitor", pgCfg.Username)
	assert.Equal(t, "/etc/.pgpass", pgCfg.Passfile)
	assert.True(t, pgCfg.Insecure)
	assert.True(t, pgCfg.InsecureSkipVerify)
	assert.Equal(t, time.Second, pgCfg.QuerySampleCollection.CollectionInterval)
	assert.Equal(t, int64(500), pgCfg.QuerySampleCollection.MaxRowsPerQuery)
	assert.Equal(t, int64(200), pgCfg.TopNQuery)
	assert.Equal(t, int64(5000), pgCfg.TopQueryCollection.MaxRowsPerQuery)
	assert.Equal(t, int64(1000), pgCfg.MaxExplainEachInterval)
}

// Explain plan collection is disabled entirely when the budget is zero, so guard
// against it silently regressing to that state.
func TestTranslator_Translate_ExplainPlansEnabled(t *testing.T) {
	tr := NewTranslator(
		WithEndpoint("localhost:5432"),
		WithUsername("cw_monitor"),
		WithPassfile("/etc/.pgpass"),
		WithIsLocalhost(true),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	pgCfg := cfg.(*postgresqlreceiver.Config)

	assert.Positive(t, pgCfg.MaxExplainEachInterval)
}

func TestTranslator_Translate_Events(t *testing.T) {
	tr := NewTranslator(
		WithName("events"),
		WithEndpoint("db.example.com:5432"),
		WithUsername("cw_monitor"),
		WithPassfile("/etc/.pgpass"),
		WithCAFile("/etc/ssl/ca.pem"),
		WithIsLocalhost(false),
		WithIndex(0),
		WithQuerySampleInterval(60*time.Second),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	pgCfg := cfg.(*postgresqlreceiver.Config)

	assert.Equal(t, "db.example.com:5432", pgCfg.Endpoint)
	assert.False(t, pgCfg.Insecure)
	assert.False(t, pgCfg.InsecureSkipVerify)
	assert.Equal(t, "/etc/ssl/ca.pem", string(pgCfg.CAFile))
	assert.Equal(t, 60*time.Second, pgCfg.QuerySampleCollection.CollectionInterval)
	assert.Equal(t, int64(500), pgCfg.QuerySampleCollection.MaxRowsPerQuery)
}

func TestTranslator_Translate_CustomInterval(t *testing.T) {
	tr := NewTranslator(
		WithEndpoint("localhost:5432"),
		WithUsername("u"),
		WithPassfile("p"),
		WithIsLocalhost(true),
		WithQuerySampleInterval(5*time.Second),
	)
	cfg, err := tr.Translate(nil)
	require.NoError(t, err)
	pgCfg := cfg.(*postgresqlreceiver.Config)

	assert.Equal(t, 5*time.Second, pgCfg.QuerySampleCollection.CollectionInterval)
	assert.Equal(t, int64(500), pgCfg.QuerySampleCollection.MaxRowsPerQuery)
}

func TestTranslator_Translate_Defaults_MetricScope(t *testing.T) {
	pgCfg := translateForTest(t, baseOpts()...)

	// Default scope leaves per-resource metrics enabled, events on, and the top-level
	// scrape interval at the upstream factory default (10s).
	assert.Equal(t, 10*time.Second, pgCfg.ControllerConfig.CollectionInterval)
	assert.True(t, pgCfg.Metrics.PostgresqlRows.Enabled)
	assert.True(t, pgCfg.Metrics.PostgresqlIndexSize.Enabled)
	assert.True(t, pgCfg.Events.DbServerQuerySample.Enabled)
	assert.True(t, pgCfg.Events.DbServerTopQuery.Enabled)
}

func TestTranslator_ServerMetricsOnly(t *testing.T) {
	pgCfg := translateForTest(t, baseOpts(WithServerMetricsOnly())...)

	// Per-resource metrics are off; server/per-DB metrics (incl. table.count) stay on;
	// events stay on; interval stays at the 10s default.
	enabled := enabledMetricNames(t, pgCfg)
	for n := range perResourceMetricNames {
		assert.False(t, enabled[n], "per-resource metric %s must be disabled on server receiver", n)
	}
	assert.True(t, pgCfg.Metrics.PostgresqlTableCount.Enabled)
	assert.True(t, pgCfg.Metrics.PostgresqlBackends.Enabled)
	assert.True(t, pgCfg.Events.DbServerQuerySample.Enabled)
	assert.Equal(t, 10*time.Second, pgCfg.ControllerConfig.CollectionInterval)
}

func TestTranslator_PerResourceMetricsOnly(t *testing.T) {
	pgCfg := translateForTest(t, baseOpts(
		WithPerResourceMetricsOnly(),
		WithCollectionInterval(60*time.Second),
		WithEventsDisabled(),
	)...)

	enabled := enabledMetricNames(t, pgCfg)
	// Every enabled metric must be a per-resource metric.
	for n := range enabled {
		assert.Contains(t, perResourceMetricNames, n, "unexpected non-per-resource metric %s enabled", n)
	}
	// Metrics enabled by default (rows, index.size, ...) are present; a per-resource
	// metric that was OFF by default (sequential_scans) stays off -- the split does not
	// introduce new metrics.
	assert.True(t, pgCfg.Metrics.PostgresqlRows.Enabled)
	assert.True(t, pgCfg.Metrics.PostgresqlIndexSize.Enabled)
	assert.False(t, pgCfg.Metrics.PostgresqlSequentialScans.Enabled)
	// table.count is a server metric and must be off here.
	assert.False(t, pgCfg.Metrics.PostgresqlTableCount.Enabled)
	// 60s interval and events disabled so it never duplicates DBLoad / TopSQL.
	assert.Equal(t, 60*time.Second, pgCfg.ControllerConfig.CollectionInterval)
	assert.False(t, pgCfg.Events.DbServerQuerySample.Enabled)
	assert.False(t, pgCfg.Events.DbServerTopQuery.Enabled)
}

// TestTranslator_MetricsDisabled covers the events receiver's scope: it is wired into a
// logs pipeline only, so it must emit no metrics while keeping the query-sample and
// top-query events that are its whole purpose.
func TestTranslator_MetricsDisabled(t *testing.T) {
	pgCfg := translateForTest(t, baseOpts(WithMetricsDisabled())...)

	assert.Empty(t, enabledMetricNames(t, pgCfg), "no metric may be enabled on a logs-only receiver")
	assert.True(t, pgCfg.Events.DbServerQuerySample.Enabled)
	assert.True(t, pgCfg.Events.DbServerTopQuery.Enabled)
}

// TestTranslator_MetricSets_Disjoint guards against double emission AND against the
// split adding/dropping metrics: the server (10s) and per-resource (60s) enabled sets
// must be disjoint, and their union must equal the default single-receiver enabled set.
func TestTranslator_MetricSets_Disjoint(t *testing.T) {
	defaultEnabled := enabledMetricNames(t, translateForTest(t, baseOpts()...))
	server := enabledMetricNames(t, translateForTest(t, baseOpts(WithServerMetricsOnly())...))
	perResource := enabledMetricNames(t, translateForTest(t, baseOpts(WithPerResourceMetricsOnly())...))

	// Disjoint: nothing enabled on both instances.
	for n := range perResource {
		assert.False(t, server[n], "metric %s is enabled on BOTH the server and per-resource receivers (double emission)", n)
	}
	// Server receiver enables no per-resource metric; per-resource receiver enables only
	// per-resource metrics.
	for n := range server {
		assert.NotContains(t, perResourceMetricNames, n, "server receiver must not enable per-resource metric %s", n)
	}
	for n := range perResource {
		assert.Contains(t, perResourceMetricNames, n, "per-resource receiver must not enable non-per-resource metric %s", n)
	}
	// Union preservation: the split neither adds nor drops any emitted metric.
	union := map[string]bool{}
	for n := range server {
		union[n] = true
	}
	for n := range perResource {
		union[n] = true
	}
	assert.Equal(t, defaultEnabled, union, "server+per-resource union must equal the default enabled metric set")
}
