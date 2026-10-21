// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// newTelemetryConnector returns a connector whose own telemetry goes to the
// returned Telemetry, with a clock at testNow and an observed debug logger.
func newTelemetryConnector(t *testing.T, cfg *Config, next consumer.Metrics) (*infoMetricConnector, *componenttest.Telemetry, *observer.ObservedLogs) {
	t.Helper()
	require.NoError(t, cfg.Validate())
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { assert.NoError(t, tel.Shutdown(context.Background())) })
	core, logs := observer.New(zapcore.DebugLevel)
	set := connectortest.NewNopSettings(component.MustNewType(TypeStr))
	set.TelemetrySettings = tel.NewTelemetrySettings()
	set.Logger = zap.New(core)
	conn, err := newConnector(set, cfg, next)
	require.NoError(t, err)
	conn.now = func() time.Time { return testNow }
	return conn, tel, logs
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

// switchConsumer accepts metrics until err is set.
type switchConsumer struct {
	countingConsumer
}

func (c *switchConsumer) setErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.err = err
}

func TestTelemetry(t *testing.T) {
	cfg := testConfig()
	cfg.MaxValueLength = 8
	cfg.Rules[0].MaxEntities = 2
	otherRule := RuleConfig{
		Name:                      "other_info",
		BodyContains:              []string{"other started"},
		Identity:                  []string{"k8s.pod.uid"},
		IncludeResourceAttributes: []string{"k8s.pod.uid"},
		Attributes:                []AttributeConfig{{Key: "version", Value: `ExtractPatterns(log.body, "version=(?P<v>\\S+)")["v"]`}},
	}
	cfg.Rules = append(cfg.Rules, otherRule)
	next := &switchConsumer{}
	conn, tel, _ := newTelemetryConnector(t, cfg, next)
	const (
		app   = "rule=app_info"
		other = "rule=other_info"
	)

	// Records that fail body_contains touch no instrument.
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "noise", "INFO request handled")))
	assert.Equal(t, map[string]map[string]int64{
		metricEntities:   {app: 0, other: 0},
		metricStateBytes: {"": 0},
	}, collect(t, tel))

	ld := podLogs("u1", base,
		"noise",
		"app started version=1",
		"app started mode=x", // version is required
		"other started version=9",
	)
	noUID := addResource(ld, map[string]string{"k8s.pod.name": "no-uid"})
	addRecord(noUID, base, "app started version=2")
	addRecord(noUID, base, "app started version=3")
	addRecord(addResource(ld, podResource("u2")), base, "app started version=123456789") // cut to 8 bytes
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))

	// u3 and u4 evict for max_entities. The emission for u4 fails.
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u3", base.Add(time.Second), "app started version=1")))
	next.setErr(errors.New("downstream unavailable"))
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u4", base.Add(2*time.Second), "app started version=1")))

	// One tick of the group of both rules that succeeds, and one that fails.
	next.setErr(nil)
	require.Len(t, conn.groups, 1)
	conn.tick(t.Context(), &conn.groups[0])
	next.setErr(errors.New("downstream unavailable"))
	conn.tick(t.Context(), &conn.groups[0])

	counts := make([]int, 2)
	bytes := conn.state.sizes(counts)
	require.Equal(t, []int{2, 1}, counts)
	assert.Equal(t, map[string]map[string]int64{
		metricRecordsMatched: {app: 7, other: 1},
		metricRecordsNotUsed: {
			"reason=identity_missing," + app: 2,
			"reason=required_missing," + app: 1,
		},
		metricValuesTruncated: {app: 1},
		metricEntitiesEvicted: {"reason=max_entities," + app: 2},
		metricDatapointsEmitted: {
			// u1, u2, then u3 on change. The failed change emission of u4 is
			// not counted.
			app + ",trigger=change":     3,
			other + ",trigger=change":   1,
			app + ",trigger=interval":   2,
			other + ",trigger=interval": 1,
		},
		metricEmitErrors: {app: 2, other: 1},
		metricEntities:   {app: 2, other: 1},
		metricStateBytes: {"": int64(bytes)},
	}, collect(t, tel))

	// Shutdown unregisters the gauges, and is safe twice.
	require.NoError(t, conn.Shutdown(t.Context()))
	require.NoError(t, conn.Shutdown(t.Context()))
	got := collect(t, tel)
	assert.NotContains(t, got, metricEntities)
	assert.NotContains(t, got, metricStateBytes)
}

func TestTelemetryStateLimits(t *testing.T) {
	cfg := testConfig()
	conn, tel, logs := newTelemetryConnector(t, cfg, consumertest.NewNop())
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "app started version=1")))
	// Room for two entities of this size.
	conn.state.maxStateBytes = conn.state.bytes*2 + conn.state.bytes/2
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u2", base.Add(time.Second), "app started version=1")))
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u3", base.Add(2*time.Second), "app started version=1")))

	// An entity that alone is too large: new, then after a change.
	huge := "app started version=" + strings.Repeat("v", 300) + " mode=" + strings.Repeat("m", 300) + " region=" + strings.Repeat("r", 300)
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u9", base.Add(3*time.Second), huge)))
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u3", base.Add(4*time.Second), huge)))

	got := collect(t, tel)
	assert.Equal(t, map[string]int64{
		"reason=max_state_bytes,rule=app_info": 1,
		"reason=too_large,rule=app_info":       2,
	}, got[metricEntitiesEvicted])
	assert.Equal(t, map[string]int64{"rule=app_info": 6}, got[metricValuesTruncated])
	warnings := logs.FilterLevelExact(zapcore.WarnLevel).All()
	require.Len(t, warnings, 1, "too_large is not a Warn")
	assert.Equal(t, "max_state_bytes", warnings[0].ContextMap()["limit"])
}

func TestTelemetryEvaluationError(t *testing.T) {
	cfg := testConfig()
	cfg.ErrorMode = ottl.PropagateError
	conn, tel, _ := newTelemetryConnector(t, cfg, consumertest.NewNop())
	// A required attribute fails to evaluate: the record matched.
	require.Error(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "app started mode=x")))

	cfg = testConfig()
	cfg.ErrorMode = ottl.PropagateError
	cfg.Rules[0].Conditions = []string{`ExtractPatterns(log.body, "level=(?P<v>\\S+)")["v"] == "info"`}
	conn2, tel2, _ := newTelemetryConnector(t, cfg, consumertest.NewNop())
	// A condition fails to evaluate: the record did not match.
	require.Error(t, conn2.ConsumeLogs(t.Context(), podLogs("u1", base, "app started version=1")))

	got := collect(t, tel)
	assert.Equal(t, map[string]int64{"rule=app_info": 1}, got[metricRecordsMatched])
	assert.Equal(t, map[string]int64{"reason=evaluation_error,rule=app_info": 1}, got[metricRecordsNotUsed])
	got = collect(t, tel2)
	assert.NotContains(t, got, metricRecordsMatched)
	assert.Equal(t, map[string]int64{"reason=evaluation_error,rule=app_info": 1}, got[metricRecordsNotUsed])

	// Without propagate, a failed required attribute is a missing one.
	conn3, tel3, _ := newTelemetryConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn3.ConsumeLogs(t.Context(), podLogs("u1", base, "app started mode=x")))
	assert.Equal(t, map[string]int64{"reason=required_missing,rule=app_info": 1}, collect(t, tel3)[metricRecordsNotUsed])
}

func TestRequiredMissingLogRateLimit(t *testing.T) {
	const msg = "Matching log record not used: a required attribute has no value"
	cfg := testConfig()
	second := cfg.Rules[0]
	second.Name = "second_info"
	cfg.Rules = append(cfg.Rules, second)
	conn, logs := newTestConnector(t, cfg, consumertest.NewNop())
	now := testNow
	conn.now = func() time.Time { return now }
	infos := func() []observer.LoggedEntry {
		return logs.FilterMessage(msg).FilterLevelExact(zapcore.InfoLevel).All()
	}
	debugs := func() int {
		return logs.FilterMessage(msg).FilterLevelExact(zapcore.DebugLevel).Len()
	}

	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "app started mode=a", "app started mode=b", "app started mode=c")))
	require.Len(t, infos(), 2, "one Info per rule")
	for i, rule := range []string{"app_info", "second_info"} {
		fields := infos()[i].ContextMap()
		assert.Equal(t, rule, fields["rule"])
		assert.Equal(t, "version", fields["attribute"])
		assert.Contains(t, fields["error"], "key not found in map")
		assert.Equal(t, int64(0), fields["suppressed_since_last_log"])
	}
	assert.Equal(t, 4, debugs(), "the other records are logged at Debug")

	now = now.Add(logInterval - time.Nanosecond)
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "app started mode=d")))
	assert.Len(t, infos(), 2)

	now = now.Add(time.Nanosecond)
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "app started mode=e")))
	require.Len(t, infos(), 4)
	assert.Equal(t, int64(3), infos()[2].ContextMap()["suppressed_since_last_log"])

	// error_mode silent logs nothing.
	cfg = testConfig()
	cfg.ErrorMode = ottl.SilentError
	conn, logs = newTestConnector(t, cfg, consumertest.NewNop())
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "app started mode=a")))
	assert.Equal(t, 0, logs.FilterMessage(msg).Len())
}

func TestEvictionLogRateLimit(t *testing.T) {
	cfg := testConfig()
	cfg.Rules[0].MaxEntities = 1
	conn, logs := newTestConnector(t, cfg, consumertest.NewNop())
	now := testNow
	conn.now = func() time.Time { return now }
	warnings := func() []observer.LoggedEntry {
		return logs.FilterLevelExact(zapcore.WarnLevel).All()
	}

	// End to end: the second pod evicts the first.
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "app started version=1")))
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u2", base.Add(time.Second), "app started version=1")))
	require.Len(t, warnings(), 1)
	assert.Equal(t, map[string]any{
		"limit":                     "max_entities",
		"rule":                      "app_info",
		"evicted":                   int64(1),
		"suppressed_since_last_log": int64(0),
	}, warnings()[0].ContextMap())
	assert.Equal(t, "Evicted the least active Info metric entities to stay within the limit. "+
		"The limit is too small only if the evicted entities were still active.", warnings()[0].Message)

	// Within the minute: max_entities is suppressed, max_state_bytes has
	// its own limit.
	evict := func(reason evictReason, n int) *batchResult {
		res := &batchResult{}
		for range n {
			res.evicted = append(res.evicted, eviction{entityRef: ref(0, "x"), reason: reason})
		}
		return res
	}
	conn.logEvictions(evict(evictMaxEntities, 3))
	conn.logEvictions(evict(evictMaxStateBytes, 2))
	conn.logEvictions(evict(evictMaxStateBytes, 1))
	conn.logEvictions(evict(evictTooLarge, 1))
	conn.logEvictions(&batchResult{rejected: []entityRef{ref(0, "y")}})
	require.Len(t, warnings(), 2)
	assert.Equal(t, "max_state_bytes", warnings()[1].ContextMap()["limit"])
	assert.Equal(t, int64(2), warnings()[1].ContextMap()["evicted"])
	debugs := logs.FilterMessage("Evicted Info metric entities").All()
	require.Len(t, debugs, 4, "the suppressed and too_large evictions are logged at Debug")
	assert.Equal(t, int64(3), debugs[0].ContextMap()["max_entities"])
	assert.Equal(t, int64(1), debugs[3].ContextMap()["too_large"])

	now = now.Add(logInterval)
	conn.logEvictions(evict(evictMaxEntities, 1))
	require.Len(t, warnings(), 3)
	assert.Equal(t, int64(3), warnings()[2].ContextMap()["suppressed_since_last_log"], "the evictions suppressed since the last Warn")
}

func TestLogLimiter(t *testing.T) {
	var l logLimiter
	start := testNow.UnixNano()
	suppressed, ok := l.allow(start, 1)
	assert.True(t, ok)
	assert.Equal(t, int64(0), suppressed)
	_, ok = l.allow(start+logInterval.Nanoseconds()-1, 2)
	assert.False(t, ok)
	_, ok = l.allow(start+logInterval.Nanoseconds()-1, 3)
	assert.False(t, ok)
	suppressed, ok = l.allow(start+logInterval.Nanoseconds(), 1)
	assert.True(t, ok)
	assert.Equal(t, int64(5), suppressed)
}
