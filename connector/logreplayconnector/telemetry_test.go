// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"context"
	"errors"
	"io/fs"
	"math"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aws/amazon-cloudwatch-agent/internal/statestore"
)

// newTelemetryConnector returns a connector whose own telemetry goes to the
// returned Telemetry, with a clock at testNow, source files that exist, and
// an observed debug logger.
func newTelemetryConnector(t *testing.T, cfg *Config, next consumer.Logs) (*replayConnector, *componenttest.Telemetry) {
	t.Helper()
	require.NoError(t, cfg.Validate())
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { assert.NoError(t, tel.Shutdown(context.Background())) })
	core, _ := observer.New(zapcore.DebugLevel)
	set := connectortest.NewNopSettings(component.MustNewType(TypeStr))
	set.TelemetrySettings = tel.NewTelemetrySettings()
	set.Logger = zap.New(core)
	set.ID = testConnectorID
	conn, err := newConnector(set, cfg, next)
	require.NoError(t, err)
	conn.now = func() time.Time { return testNow }
	conn.checkSource = func(string) error { return nil }
	conn.recheckDelay = 0
	return conn, tel
}

// collect returns the int64 data points of each metric of the connector
// scope, by metric name and then by attributes ("k=v,k=v" sorted by key).
// Metrics without data are absent.
func collect(t *testing.T, tel *componenttest.Telemetry) map[string]map[string]int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, tel.Reader.Collect(context.Background(), &rm))
	out := map[string]map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		require.Equal(t, scopeName, sm.Scope.Name)
		for _, m := range sm.Metrics {
			points := map[string]int64{}
			switch d := m.Data.(type) {
			case metricdata.Sum[int64]:
				assert.True(t, d.IsMonotonic, m.Name)
				for _, dp := range d.DataPoints {
					points[attrString(dp.Attributes)] = dp.Value
				}
			case metricdata.Gauge[int64]:
				for _, dp := range d.DataPoints {
					points[attrString(dp.Attributes)] = dp.Value
				}
			default:
				t.Fatalf("%s: unexpected data type %T", m.Name, m.Data)
			}
			if len(points) > 0 {
				out[m.Name] = points
			}
		}
	}
	return out
}

func attrString(set attribute.Set) string {
	var parts []string
	for _, kv := range set.ToSlice() {
		parts = append(parts, string(kv.Key)+"="+kv.Value.String())
	}
	return strings.Join(parts, ",")
}

func TestTelemetry(t *testing.T) {
	cfg := testConfig()
	cfg.MaxRecordBytes = minMaxRecordBytes
	cfg.Rules[0].MaxEntries = 2
	next := &errConsumer{}
	conn, tel := newTelemetryConnector(t, cfg, next)
	files := newFakeFiles()
	conn.checkSource = files.stat
	ctx := t.Context()

	ld := mergeLogs(
		podLogs("p1", base, "app started", "request"),
		podLogs("p2", base.Add(time.Second), "app started"),
		podLogs("p3", base.Add(2*time.Second), "app started"),
		podLogs("big", base, "app started "+strings.Repeat("x", minMaxRecordBytes)),
	)
	noID := podResource("x")
	delete(noID, "k8s.pod.name")
	addRecord(addResource(ld, noID), base, "app started", podFile("x"))
	addRecord(addResource(ld, podResource("nofile")), base, "app started", "")
	require.NoError(t, conn.ConsumeLogs(ctx, ld))

	files.remove(podFile("p2"))
	files.setErr(podFile("p3"), fs.ErrPermission)
	conn.tick(ctx)
	files.setErr(podFile("p3"), nil)
	next.err = errors.New("throttled")
	conn.tick(ctx)
	next.err = nil
	conn.tick(ctx)

	rule := "rule=app_start"
	assert.Equal(t, map[string]map[string]int64{
		metricRecordsMatched: {rule: 6},
		metricRecordsNotUsed: {
			"reason=identity_missing," + rule:    1,
			"reason=source_file_missing," + rule: 1,
			"reason=too_large," + rule:           1,
		},
		// p1 is evicted for p3 at max_entries 2. At the first tick, p2 is
		// gone and p3 cannot be checked. The second tick fails, and the
		// third sends p3.
		metricRecordsReplayed: {rule: 1},
		metricRecordsSkipped:  {"reason=source_check_error," + rule: 1},
		metricReplayErrors:    {rule: 1},
		metricEntriesRemoved: {
			"reason=max_entries," + rule: 1,
			"reason=source_gone," + rule: 1,
		},
		metricEntries:    {rule: 1},
		metricStateBytes: {"": int64(stateBytes(conn))},
	}, collect(t, tel))
}

func stateBytes(conn *replayConnector) int {
	_, bytes := conn.state.sizes()
	return bytes
}

func TestTelemetryEvaluationError(t *testing.T) {
	cfg := testConfig()
	cfg.ErrorMode = "propagate"
	cfg.Rules[0].Conditions = []string{`ExtractPatterns(log.body, "level=(?P<v>\\S+)")["v"] == "info"`}
	conn, tel := newTelemetryConnector(t, cfg, consumertest.NewNop())
	require.Error(t, conn.ConsumeLogs(t.Context(), podLogs("p1", base, "app started")))
	got := collect(t, tel)
	assert.Equal(t, map[string]int64{"reason=evaluation_error,rule=app_start": 1}, got[metricRecordsNotUsed])
	assert.NotContains(t, got, metricRecordsMatched)
}

func TestTelemetryStore(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		m := newMemStorage()
		cfg := testConfig()
		cfg.Storage = &testStorageID
		conn, tel := newTelemetryConnector(t, cfg, consumertest.NewNop())
		require.NoError(t, conn.Start(ctx, m.host()))
		require.NoError(t, conn.ConsumeLogs(ctx, podLogs("p1", base, "app started")))
		time.Sleep(2 * time.Second)
		synctest.Wait()
		got := collect(t, tel)
		assert.Equal(t, map[string]int64{"": 1}, got[metricStateSaves])
		assert.Positive(t, got[metricStateSavedBytes][""])
		assert.Equal(t, map[string]int64{"": 0}, got[metricStateSaveErrors])
		require.NoError(t, conn.Shutdown(ctx))
		assert.NotContains(t, collect(t, tel), metricStateSaves, "Shutdown unregisters the callbacks")
	})
}

func TestTelemetryRegisterStoreAfterShutdown(t *testing.T) {
	conn, tel := newTelemetryConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn.tel.registerStore(func() statestore.Stats { return statestore.Stats{Saves: 3} }))
	assert.Equal(t, map[string]int64{"": 3}, collect(t, tel)[metricStateSaves])
	conn.tel.shutdown()
	conn.tel.shutdown()
	assert.NotContains(t, collect(t, tel), metricStateSaves)
}

func TestClampInt64(t *testing.T) {
	assert.Equal(t, int64(5), clampInt64(5))
	assert.Equal(t, int64(math.MaxInt64), clampInt64(math.MaxUint64))
}

func TestLogLimiter(t *testing.T) {
	var l logLimiter
	now := testNow.UnixNano()
	_, ok := l.allow(now, 1)
	assert.True(t, ok)
	_, ok = l.allow(now+int64(time.Second), 3)
	assert.False(t, ok)
	suppressed, ok := l.allow(now+int64(time.Minute), 1)
	assert.True(t, ok)
	assert.Equal(t, int64(3), suppressed)
}

// TestTelemetryCountsNothingWithoutMatch checks that records that fail
// body_contains are not counted.
func TestTelemetryCountsNothingWithoutMatch(t *testing.T) {
	conn, tel := newTelemetryConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("p1", base, "request")))
	require.NoError(t, conn.ConsumeLogs(t.Context(), plog.NewLogs()))
	got := collect(t, tel)
	assert.Equal(t, map[string]map[string]int64{
		metricEntries:    {"rule=app_start": 0},
		metricStateBytes: {"": 0},
	}, got)
}
