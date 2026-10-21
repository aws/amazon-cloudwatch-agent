// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// testNow is the emit time in tests.
var testNow = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

// base is a record time in tests.
var base = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

// testConfig returns a valid configuration with one rule, app_info, that
// matches "app started" lines. version is required, mode has a default,
// and region is optional.
func testConfig() *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.Rules = []RuleConfig{{
		Name:                      "app_info",
		Description:               "App configuration.",
		Unit:                      "1",
		BodyContains:              []string{"app started"},
		Identity:                  []string{"k8s.pod.uid"},
		IncludeResourceAttributes: []string{"k8s.pod.name", "k8s.pod.uid"},
		Attributes: []AttributeConfig{
			{Key: "version", Value: `ExtractPatterns(log.body, "version=(?P<v>\\S+)")["v"]`},
			{Key: "mode", Value: `ExtractPatterns(log.body, "mode=(?P<v>\\S+)")["v"]`, DefaultValue: "default"},
			{Key: "region", Value: `ExtractPatterns(log.body, "region=(?P<v>\\S+)")["v"]`, Optional: true},
		},
	}}
	return cfg
}

// newTestConnector validates cfg and creates a connector whose clock is
// testNow and whose logger is observed at debug level.
func newTestConnector(t testing.TB, cfg *Config, next consumer.Metrics) (*infoMetricConnector, *observer.ObservedLogs) {
	t.Helper()
	require.NoError(t, cfg.Validate())
	core, logs := observer.New(zapcore.DebugLevel)
	set := connectortest.NewNopSettings(component.MustNewType(TypeStr))
	set.Logger = zap.New(core)
	conn, err := newConnector(set, cfg, next)
	require.NoError(t, err)
	conn.now = func() time.Time { return testNow }
	return conn, logs
}

// podResource returns resource attributes for a pod.
func podResource(uid string) map[string]string {
	return map[string]string{"k8s.pod.uid": uid, "k8s.pod.name": "pod-" + uid}
}

// addResource appends a ResourceLogs with one ScopeLogs and returns its records.
func addResource(ld plog.Logs, attrs map[string]string) plog.LogRecordSlice {
	rl := ld.ResourceLogs().AppendEmpty()
	for k, v := range attrs {
		rl.Resource().Attributes().PutStr(k, v)
	}
	return rl.ScopeLogs().AppendEmpty().LogRecords()
}

// addRecord appends a record with a string body; a zero ts leaves the
// timestamp unset.
func addRecord(lrs plog.LogRecordSlice, ts time.Time, body string) plog.LogRecord {
	lr := lrs.AppendEmpty()
	lr.Body().SetStr(body)
	if !ts.IsZero() {
		lr.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	}
	return lr
}

// podLogs returns logs with one pod and one record per body.
func podLogs(uid string, ts time.Time, bodies ...string) plog.Logs {
	ld := plog.NewLogs()
	lrs := addResource(ld, podResource(uid))
	for _, body := range bodies {
		addRecord(lrs, ts, body)
	}
	return ld
}

// series is one emitted data point, flattened for comparison.
type series struct {
	resource       []kv
	name           string
	start          time.Time
	ts             time.Time
	attrs          []kv
	value          int64
	scopeName      string
	description    string
	unit           string
	dataPointCount int
}

func pairsOf(m pcommon.Map) []kv {
	var out []kv
	for k, v := range m.All() {
		out = append(out, kv{Key: k, Value: v.AsString()})
	}
	return out
}

// allSeries flattens every gauge data point in the metrics.
func allSeries(t *testing.T, mds []pmetric.Metrics) []series {
	t.Helper()
	var out []series
	for _, md := range mds {
		for _, rm := range md.ResourceMetrics().All() {
			require.Equal(t, 1, rm.ScopeMetrics().Len())
			sm := rm.ScopeMetrics().At(0)
			require.Equal(t, 1, sm.Metrics().Len())
			m := sm.Metrics().At(0)
			require.Equal(t, pmetric.MetricTypeGauge, m.Type())
			dps := m.Gauge().DataPoints()
			require.Equal(t, 1, dps.Len())
			dp := dps.At(0)
			require.Equal(t, pmetric.NumberDataPointValueTypeInt, dp.ValueType())
			out = append(out, series{
				resource:       pairsOf(rm.Resource().Attributes()),
				name:           m.Name(),
				start:          dp.StartTimestamp().AsTime(),
				ts:             dp.Timestamp().AsTime(),
				attrs:          pairsOf(dp.Attributes()),
				value:          dp.IntValue(),
				scopeName:      sm.Scope().Name(),
				description:    m.Description(),
				unit:           m.Unit(),
				dataPointCount: md.DataPointCount(),
			})
		}
	}
	return out
}

func newTCtx(lr plog.LogRecord) *ottllog.TransformContext {
	return ottllog.NewTransformContextPtr(plog.NewResourceLogs(), plog.NewScopeLogs(), lr)
}

// sameMemory reports whether b points into the memory of a.
func sameMemory(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	pa := uintptr(unsafe.Pointer(unsafe.StringData(a))) //nolint:gosec // the test checks memory aliasing
	pb := uintptr(unsafe.Pointer(unsafe.StringData(b))) //nolint:gosec // the test checks memory aliasing
	return pb >= pa && pb < pa+uintptr(len(a))
}

func TestConsumeLogsEmitsOnInsert(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	conn, _ := newTestConnector(t, testConfig(), sink)
	require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))
	assert.Empty(t, sink.AllMetrics(), "Start does not emit")

	ts := base.Add(time.Second)
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", ts, "noise", "app started version=1.2.3 mode=fast region=eu", "noise")))
	got := allSeries(t, sink.AllMetrics())
	require.Len(t, got, 1)
	assert.Equal(t, series{
		resource:       []kv{{"k8s.pod.name", "pod-u1"}, {"k8s.pod.uid", "u1"}},
		name:           "app_info",
		start:          ts,
		ts:             testNow,
		attrs:          []kv{{"version", "1.2.3"}, {"mode", "fast"}, {"region", "eu"}},
		value:          1,
		scopeName:      scopeName,
		description:    "App configuration.",
		unit:           "1",
		dataPointCount: 1,
	}, got[0])
	require.NoError(t, conn.Shutdown(t.Context()))
}

func TestConsumeLogsNoSeries(t *testing.T) {
	testCases := []struct {
		name string
		// edit changes the default test configuration.
		edit  func(*Config)
		build func() plog.Logs
	}{
		{name: "noise", build: func() plog.Logs {
			return podLogs("u1", base, "", "INFO request handled", `{"msg":"app stopped"}`, "App Started version=1")
		}},
		{name: "required_fact_missing", build: func() plog.Logs {
			return podLogs("u1", base, "app started mode=fast region=eu")
		}},
		{name: "missing_identity", build: func() plog.Logs {
			ld := plog.NewLogs()
			addRecord(addResource(ld, map[string]string{"k8s.pod.name": "pod-u1"}), base, "app started version=1")
			return ld
		}},
		{name: "empty_identity", build: func() plog.Logs {
			ld := plog.NewLogs()
			addRecord(addResource(ld, map[string]string{"k8s.pod.uid": ""}), base, "app started version=1")
			return ld
		}},
		{name: "bytes_body", build: func() plog.Logs {
			ld := podLogs("u1", base)
			lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
			lr.Body().SetEmptyBytes().FromRaw([]byte("app started version=1"))
			return ld
		}},
		{name: "map_body", build: func() plog.Logs {
			ld := podLogs("u1", base)
			lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
			lr.Body().SetEmptyMap().PutStr("message", "app started version=1")
			return ld
		}},
		{
			name: "body_contains_passes_condition_fails",
			edit: func(cfg *Config) { cfg.Rules[0].Conditions = []string{`attributes["level"] == "info"`} },
			build: func() plog.Logs {
				ld := podLogs("u1", base, "app started version=1")
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("level", "debug")
				return ld
			},
		},
		{
			name: "condition_passes_body_contains_fails",
			edit: func(cfg *Config) { cfg.Rules[0].Conditions = []string{`attributes["level"] == "info"`} },
			build: func() plog.Logs {
				ld := podLogs("u1", base, "service ready version=1")
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("level", "info")
				return ld
			},
		},
		{
			name: "non_string_body_with_condition",
			edit: func(cfg *Config) { cfg.Rules[0].Conditions = []string{`true`} },
			build: func() plog.Logs {
				ld := podLogs("u1", base)
				lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
				lr.Body().SetEmptyMap().PutStr("message", "app started version=1")
				return ld
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			if tc.edit != nil {
				tc.edit(cfg)
			}
			sink := new(consumertest.MetricsSink)
			conn, _ := newTestConnector(t, cfg, sink)
			require.NoError(t, conn.ConsumeLogs(t.Context(), tc.build()))
			assert.Empty(t, sink.AllMetrics())
			assert.Empty(t, conn.state.snapshot().Rules)
		})
	}
}

func TestConsumeLogsMatching(t *testing.T) {
	testCases := []struct {
		name         string
		bodyContains []string
		conditions   []string
		body         string
		level        string
		want         bool
	}{
		{name: "body_contains_only", bodyContains: []string{"app started"}, body: "app started version=1", want: true},
		{name: "body_contains_any", bodyContains: []string{"nope", "started"}, body: "app started version=1", want: true},
		{name: "body_contains_none", bodyContains: []string{"nope", "stopped"}, body: "app started version=1"},
		{name: "conditions_only", conditions: []string{`attributes["level"] == "info"`}, body: "version=1", level: "info", want: true},
		{name: "conditions_ored", conditions: []string{`attributes["level"] == "warn"`, `attributes["level"] == "info"`}, body: "version=1", level: "info", want: true},
		{name: "conditions_false", conditions: []string{`attributes["level"] == "warn"`}, body: "version=1", level: "info"},
		{name: "both_pass", bodyContains: []string{"started"}, conditions: []string{`attributes["level"] == "info"`}, body: "app started version=1", level: "info", want: true},
		{name: "both_body_fails", bodyContains: []string{"stopped"}, conditions: []string{`attributes["level"] == "info"`}, body: "app started version=1", level: "info"},
		{name: "both_condition_fails", bodyContains: []string{"started"}, conditions: []string{`attributes["level"] == "warn"`}, body: "app started version=1", level: "info"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.Rules[0].BodyContains = tc.bodyContains
			cfg.Rules[0].Conditions = tc.conditions
			sink := new(consumertest.MetricsSink)
			conn, _ := newTestConnector(t, cfg, sink)
			ld := podLogs("u1", base, tc.body)
			if tc.level != "" {
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("level", tc.level)
			}
			require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
			assert.Equal(t, tc.want, len(sink.AllMetrics()) == 1)
		})
	}
}

// TestConsumeLogsFacts covers default_value, optional, required, value
// conversion, and truncation, with facts read from log attributes.
func TestConsumeLogsFacts(t *testing.T) {
	testCases := []struct {
		name string
		// maxLen is max_value_length; 0 means 4.
		maxLen int
		attrs  func(pcommon.Map)
		// want is nil when the record yields no series.
		want []kv
	}{
		{name: "all", attrs: func(m pcommon.Map) { m.PutStr("req", "r"); m.PutStr("def", "x"); m.PutStr("opt", "o") },
			want: []kv{{"req", "r"}, {"def", "x"}, {"opt", "o"}}},
		{name: "default_and_optional_absent", attrs: func(m pcommon.Map) { m.PutStr("req", "r") },
			want: []kv{{"req", "r"}, {"def", "d"}}},
		{name: "empty_strings_are_missing", attrs: func(m pcommon.Map) { m.PutStr("req", "r"); m.PutStr("def", ""); m.PutStr("opt", "") },
			want: []kv{{"req", "r"}, {"def", "d"}}},
		{name: "empty_value_is_missing", attrs: func(m pcommon.Map) { m.PutStr("req", "r"); m.PutEmpty("def"); m.PutEmpty("opt") },
			want: []kv{{"req", "r"}, {"def", "d"}}},
		{name: "required_absent", attrs: func(m pcommon.Map) { m.PutStr("def", "x"); m.PutStr("opt", "o") }},
		{name: "required_empty", attrs: func(m pcommon.Map) { m.PutStr("req", "") }},
		{name: "int", attrs: func(m pcommon.Map) { m.PutInt("req", 4) }, want: []kv{{"req", "4"}, {"def", "d"}}},
		{name: "bool", attrs: func(m pcommon.Map) { m.PutBool("req", true) }, want: []kv{{"req", "true"}, {"def", "d"}}},
		{name: "double", attrs: func(m pcommon.Map) { m.PutDouble("req", 1.5) }, want: []kv{{"req", "1.5"}, {"def", "d"}}},
		{name: "map", attrs: func(m pcommon.Map) { m.PutEmptyMap("req").PutInt("a", 1) }, want: []kv{{"req", `{"a"`}, {"def", "d"}}},
		{name: "truncated_ascii", attrs: func(m pcommon.Map) { m.PutStr("req", "abcdef") }, want: []kv{{"req", "abcd"}, {"def", "d"}}},
		{name: "truncated_utf8", attrs: func(m pcommon.Map) { m.PutStr("req", "abcé"); m.PutStr("opt", "日本") },
			want: []kv{{"req", "abc"}, {"def", "d"}, {"opt", "日"}}},
		{name: "truncated_to_empty_is_missing", maxLen: 1, attrs: func(m pcommon.Map) { m.PutStr("req", "r"); m.PutStr("def", "é"); m.PutStr("opt", "é") },
			want: []kv{{"req", "r"}, {"def", "d"}}},
		{name: "required_truncated_to_empty", maxLen: 1, attrs: func(m pcommon.Map) { m.PutStr("req", "é") }},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.MaxValueLength = cmp.Or(tc.maxLen, 4)
			cfg.Rules[0].Attributes = []AttributeConfig{
				{Key: "req"},
				{Key: "def", DefaultValue: "d"},
				{Key: "opt", Optional: true},
			}
			sink := new(consumertest.MetricsSink)
			conn, _ := newTestConnector(t, cfg, sink)
			ld := podLogs("u1", base, "app started")
			tc.attrs(ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes())
			require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
			got := allSeries(t, sink.AllMetrics())
			if tc.want == nil {
				assert.Empty(t, got)
				return
			}
			require.Len(t, got, 1)
			assert.Equal(t, tc.want, got[0].attrs)
		})
	}
}

func TestConsumeLogsTruncatesResourceAndIdentity(t *testing.T) {
	cfg := testConfig()
	cfg.MaxValueLength = 5
	cfg.Rules[0].Attributes[1].DefaultValue = "dflt"
	sink := new(consumertest.MetricsSink)
	conn, _ := newTestConnector(t, cfg, sink)
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("uid-éé", base, "app started version=1.2.3")))
	got := allSeries(t, sink.AllMetrics())
	require.Len(t, got, 1)
	assert.Equal(t, []kv{{"k8s.pod.name", "pod-u"}, {"k8s.pod.uid", "uid-"}}, got[0].resource)
	assert.Equal(t, []kv{{"version", "1.2.3"}, {"mode", "dflt"}}, got[0].attrs)
}

// TestConsumeLogsInvalidUTF8 checks that invalid UTF-8 in a fact, a
// resource value, and an identity value is replaced with U+FFFD before the
// cut, so that every emitted value is valid UTF-8 within max_value_length
// and the metrics marshal to OTLP protobuf.
func TestConsumeLogsInvalidUTF8(t *testing.T) {
	for _, tc := range []struct {
		name     string
		maxLen   int
		resource map[string]string
		body     string
		// wantUID is the identity value of the state key.
		wantUID      string
		wantResource []kv
		wantAttrs    []kv
	}{
		{
			name:         "replaced",
			maxLen:       256,
			resource:     map[string]string{"k8s.pod.uid": "u\xff1", "k8s.pod.name": "pod-\xfe\xfd"},
			body:         "app started version=1.\x80 mode=f\xc3ast region=eu",
			wantUID:      "u\uFFFD1",
			wantResource: []kv{{"k8s.pod.name", "pod-\uFFFD"}, {"k8s.pod.uid", "u\uFFFD1"}},
			wantAttrs:    []kv{{"version", "1.\uFFFD"}, {"mode", "f\uFFFDast"}, {"region", "eu"}},
		},
		{
			// The replacement makes "abc\x80" 6 bytes and "pod-u\x80" 8,
			// and the cut brings them back within 4. It makes the region 3
			// bytes, from 5, so the region is not cut.
			name:         "replaced_then_cut",
			maxLen:       4,
			resource:     map[string]string{"k8s.pod.uid": "u\x80", "k8s.pod.name": "pod-u\x80"},
			body:         "app started version=abc\x80 region=\x80\x80\x80\x80\x80",
			wantUID:      "u\uFFFD",
			wantResource: []kv{{"k8s.pod.name", "pod-"}, {"k8s.pod.uid", "u\uFFFD"}},
			wantAttrs:    []kv{{"version", "abc"}, {"mode", "d"}, {"region", "\uFFFD"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.MaxValueLength = tc.maxLen
			cfg.Rules[0].Attributes[1].DefaultValue = "d"
			sink := new(consumertest.MetricsSink)
			conn, _ := newTestConnector(t, cfg, sink)
			ld := plog.NewLogs()
			addRecord(addResource(ld, tc.resource), base, tc.body)
			require.NoError(t, conn.ConsumeLogs(t.Context(), ld))

			got := allSeries(t, sink.AllMetrics())
			require.Len(t, got, 1)
			assert.Equal(t, tc.wantResource, got[0].resource)
			assert.Equal(t, tc.wantAttrs, got[0].attrs)
			for _, p := range slices.Concat(got[0].resource, got[0].attrs) {
				assert.LessOrEqual(t, len(p.Value), tc.maxLen, p.Key)
			}
			assert.Contains(t, conn.state.snapshot().Rules["app_info"], key(tc.wantUID))
			assertValidOTLP(t, sink.AllMetrics()[0])
		})
	}
}

// assertValidOTLP checks that every string of md is valid UTF-8, and that md
// marshals to OTLP protobuf and back. pdata does not check UTF-8 when it
// marshals, so the check of the strings is what shows that a receiver that
// checks UTF-8, as protobuf requires for a string field, accepts md.
func assertValidOTLP(t *testing.T, md pmetric.Metrics) {
	t.Helper()
	validMap := func(m pcommon.Map) {
		for k, v := range m.All() {
			assert.True(t, utf8.ValidString(k), "key %q", k)
			assert.True(t, utf8.ValidString(v.AsString()), "value %q of %s", v.AsString(), k)
		}
	}
	for _, rm := range md.ResourceMetrics().All() {
		validMap(rm.Resource().Attributes())
		for _, sm := range rm.ScopeMetrics().All() {
			assert.True(t, utf8.ValidString(sm.Scope().Name()))
			for _, m := range sm.Metrics().All() {
				for _, s := range []string{m.Name(), m.Description(), m.Unit()} {
					assert.True(t, utf8.ValidString(s), "%q", s)
				}
				for _, dp := range m.Gauge().DataPoints().All() {
					validMap(dp.Attributes())
				}
			}
		}
	}
	b, err := (&pmetric.ProtoMarshaler{}).MarshalMetrics(md)
	require.NoError(t, err)
	back, err := (&pmetric.ProtoUnmarshaler{}).UnmarshalMetrics(b)
	require.NoError(t, err)
	assert.Equal(t, allSeries(t, []pmetric.Metrics{md}), allSeries(t, []pmetric.Metrics{back}))
}

func TestConsumeLogsFactsChange(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	conn, _ := newTestConnector(t, testConfig(), sink)
	t1, t2 := base, base.Add(time.Hour)

	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", t1, "app started version=1")))
	// The same facts again: no second emission.
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", t1.Add(time.Minute), "app started version=1")))
	require.Len(t, sink.AllMetrics(), 1)
	// New facts: one series with the new labels and the new start time.
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", t2, "app started version=2 region=eu")))
	got := allSeries(t, sink.AllMetrics())
	require.Len(t, got, 2)
	assert.Equal(t, []kv{{"version", "1"}, {"mode", "default"}}, got[0].attrs)
	assert.Equal(t, t1, got[0].start)
	assert.Equal(t, []kv{{"version", "2"}, {"mode", "default"}, {"region", "eu"}}, got[1].attrs)
	assert.Equal(t, t2, got[1].start)
	assert.Equal(t, testNow, got[1].ts)

	// One entity, holding the new facts.
	snap := conn.state.snapshot()
	require.Len(t, snap.Rules["app_info"], 1)

	// The older record is ignored: no emission, facts unchanged.
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", t1.Add(2*time.Minute), "app started version=1")))
	assert.Len(t, sink.AllMetrics(), 2)
	e := conn.state.snapshot().Rules["app_info"][key("u1")]
	assert.Equal(t, []kv{{"version", "2"}, {"mode", "default"}, {"region", "eu"}}, e.Facts)
	assert.Equal(t, t2.UnixNano(), e.FirstSeen)
}

func TestConsumeLogsBatchEmitsFinalValue(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	conn, _ := newTestConnector(t, testConfig(), sink)
	ld := plog.NewLogs()
	lrs := addResource(ld, podResource("u1"))
	// Read in the order 1.log then 0.log: the newer facts come first.
	addRecord(lrs, base.Add(time.Hour), "app started version=new")
	addRecord(lrs, base, "app started version=old")
	lrs = addResource(ld, podResource("u2"))
	addRecord(lrs, base, "app started version=a")
	addRecord(lrs, base.Add(time.Second), "app started version=b")
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))

	require.Len(t, sink.AllMetrics(), 1, "one ConsumeMetrics call per batch")
	got := allSeries(t, sink.AllMetrics())
	require.Len(t, got, 2, "one series per entity")
	assert.Equal(t, "new", got[0].attrs[0].Value)
	assert.Equal(t, base.Add(time.Hour), got[0].start)
	assert.Equal(t, "b", got[1].attrs[0].Value)
	assert.Equal(t, base.Add(time.Second), got[1].start)
}

func TestConsumeLogsRecordTime(t *testing.T) {
	ts, observed := base.Add(time.Minute), base.Add(2*time.Minute)
	testCases := []struct {
		name     string
		ts       time.Time
		observed time.Time
		want     time.Time
	}{
		{name: "timestamp", ts: ts, observed: observed, want: ts},
		{name: "observed_timestamp", observed: observed, want: observed},
		{name: "now", want: testNow},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sink := new(consumertest.MetricsSink)
			conn, _ := newTestConnector(t, testConfig(), sink)
			ld := podLogs("u1", tc.ts, "app started version=1")
			if !tc.observed.IsZero() {
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).SetObservedTimestamp(pcommon.NewTimestampFromTime(tc.observed))
			}
			require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
			got := allSeries(t, sink.AllMetrics())
			require.Len(t, got, 1)
			assert.Equal(t, tc.want, got[0].start)
			assert.Equal(t, testNow, got[0].ts, "the data point time is the emit time")
		})
	}
}

func TestConsumeLogsIdentityResolvedOncePerResource(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	conn, logs := newTestConnector(t, testConfig(), sink)
	ld := plog.NewLogs()
	lrs := addResource(ld, map[string]string{"k8s.pod.name": "no-uid"})
	addRecord(lrs, base, "app started version=1")
	addRecord(lrs, base, "app started version=2")
	// A second ResourceLogs resolves its own identity.
	addRecord(addResource(ld, podResource("u1")), base, "app started version=3")
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))

	assert.Equal(t, 1, logs.FilterMessageSnippet("an identity attribute is missing").Len())
	got := allSeries(t, sink.AllMetrics())
	require.Len(t, got, 1)
	assert.Equal(t, "3", got[0].attrs[0].Value)
}

func TestConsumeLogsMultipleRules(t *testing.T) {
	cfg := testConfig()
	second := cfg.Rules[0]
	second.Name = "app_mode_info"
	second.Attributes = []AttributeConfig{{Key: "mode", Value: `ExtractPatterns(log.body, "mode=(?P<v>\\S+)")["v"]`}}
	second.MaxEntities = 1
	cfg.Rules = append(cfg.Rules, second)
	sink := new(consumertest.MetricsSink)
	conn, _ := newTestConnector(t, cfg, sink)

	ld := podLogs("u1", base, "app started version=1 mode=fast", "app started version=2")
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
	got := allSeries(t, sink.AllMetrics())
	require.Len(t, got, 2)
	assert.Equal(t, "app_info", got[0].name)
	assert.Equal(t, []kv{{"version", "2"}, {"mode", "default"}}, got[0].attrs)
	assert.Equal(t, "app_mode_info", got[1].name)
	assert.Equal(t, []kv{{"mode", "fast"}}, got[1].attrs)

	// max_entities is per rule: u2 evicts u1 from app_mode_info only.
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u2", base.Add(time.Second), "app started version=1 mode=slow")))
	snap := conn.state.snapshot()
	assert.Len(t, snap.Rules["app_info"], 2)
	assert.Len(t, snap.Rules["app_mode_info"], 1)
	assert.Contains(t, snap.Rules["app_mode_info"], key("u2"))
}

func TestConsumeLogsEmitOnChangeDisabled(t *testing.T) {
	cfg := testConfig()
	cfg.EmitOnChange = false
	next := &countingConsumer{}
	conn, _ := newTestConnector(t, cfg, next)
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "app started version=1")))
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base.Add(time.Second), "app started version=2")))
	assert.Equal(t, 0, next.calls())
	// The state is still kept, for the periodic emission.
	e := conn.state.snapshot().Rules["app_info"][key("u1")]
	assert.Equal(t, "2", e.Facts[0].Value)
}

func TestConsumeLogsDownstreamErrorIsSwallowed(t *testing.T) {
	next := &countingConsumer{err: errors.New("downstream unavailable")}
	conn, logs := newTestConnector(t, testConfig(), next)
	now := testNow
	conn.now = func() time.Time { return now }

	warnings := func() []observer.LoggedEntry {
		return logs.FilterMessage("Failed to send Info metrics to the next consumer").All()
	}
	for i := range 3 {
		ld := podLogs(fmt.Sprintf("u%d", i), base, "app started version=1")
		assert.NoError(t, conn.ConsumeLogs(t.Context(), ld))
	}
	assert.Equal(t, 3, next.calls())
	require.Len(t, warnings(), 1, "logged at most once a minute")
	assert.Equal(t, "downstream unavailable", warnings()[0].ContextMap()["error"])
	assert.Equal(t, int64(0), warnings()[0].ContextMap()["suppressed_since_last_log"])

	now = now.Add(logInterval)
	assert.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u9", base, "app started version=1")))
	require.Len(t, warnings(), 2)
	assert.Equal(t, int64(2), warnings()[1].ContextMap()["suppressed_since_last_log"])

	// The state was updated even though the emission failed.
	assert.Len(t, conn.state.snapshot().Rules["app_info"], 4)
}

func TestConsumeLogsErrorMode(t *testing.T) {
	// The second record has no version, so the required expression fails
	// with "key not found in map".
	build := func() plog.Logs {
		ld := podLogs("u1", base, "app started version=1")
		addRecord(addResource(ld, podResource("u2")), base, "app started")
		addRecord(addResource(ld, podResource("u3")), base, "app started version=3")
		return ld
	}
	for _, mode := range []ottl.ErrorMode{ottl.PropagateError, ottl.IgnoreError, ottl.SilentError} {
		t.Run(string(mode), func(t *testing.T) {
			cfg := testConfig()
			cfg.ErrorMode = mode
			sink := new(consumertest.MetricsSink)
			conn, logs := newTestConnector(t, cfg, sink)
			err := conn.ConsumeLogs(t.Context(), build())
			skipped := logs.FilterMessageSnippet("a required attribute has no value").Len()
			snap := conn.state.snapshot().Rules["app_info"]
			if mode == ottl.PropagateError {
				require.ErrorContains(t, err, `rule "app_info": failed to evaluate required attribute "version"`)
				assert.ErrorContains(t, err, "key not found in map")
				// Records before the failing one are applied and emitted;
				// the rest of the batch is not processed.
				assert.Len(t, snap, 1)
				assert.Contains(t, snap, key("u1"))
				assert.Len(t, allSeries(t, sink.AllMetrics()), 1)
				return
			}
			require.NoError(t, err)
			assert.Len(t, snap, 2)
			assert.NotContains(t, snap, key("u2"))
			if mode == ottl.SilentError {
				assert.Equal(t, 0, skipped)
			} else {
				assert.Equal(t, 1, skipped)
			}
		})
	}

	// Missing optional and defaulted facts are not errors in propagate mode,
	// even though their expressions fail.
	cfg := testConfig()
	cfg.ErrorMode = ottl.PropagateError
	sink := new(consumertest.MetricsSink)
	conn, _ := newTestConnector(t, cfg, sink)
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "app started version=1")))
	assert.Len(t, sink.AllMetrics(), 1)

	// A condition evaluation error propagates too.
	cfg = testConfig()
	cfg.ErrorMode = ottl.PropagateError
	cfg.Rules[0].Conditions = []string{`ExtractPatterns(log.body, "level=(?P<v>\\S+)")["v"] == "info"`}
	conn, _ = newTestConnector(t, cfg, new(consumertest.MetricsSink))
	assert.ErrorContains(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "app started version=1")), `rule "app_info": failed to eval condition`)
	cfg.ErrorMode = ottl.IgnoreError
	conn, _ = newTestConnector(t, cfg, new(consumertest.MetricsSink))
	assert.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "app started version=1")))
}

// TestConsumeLogsReadOnlyInput shows that the connector does not modify its
// input: pdata panics on any change to read-only data.
func TestConsumeLogsReadOnlyInput(t *testing.T) {
	cfg := testConfig()
	cfg.Rules[0].Conditions = []string{`attributes["level"] == "info" or IsMatch(body, "started")`}
	cfg.Rules[0].Attributes = append(cfg.Rules[0].Attributes,
		AttributeConfig{Key: "level"},
		AttributeConfig{Key: "body_len", Value: `Len(log.body)`},
		AttributeConfig{Key: "resource_name", Value: `resource.attributes["k8s.pod.name"]`},
	)
	sink := new(consumertest.MetricsSink)
	conn, _ := newTestConnector(t, cfg, sink)
	const body = "app started version=1 mode=fast"
	ld := podLogs("u1", base, "noise", body, "app started")
	ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("level", "info")
	marshaler := &plog.ProtoMarshaler{}
	want, err := marshaler.MarshalLogs(ld)
	require.NoError(t, err)
	ld.MarkReadOnly()

	require.NotPanics(t, func() {
		require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
	})
	got := allSeries(t, sink.AllMetrics())
	require.Len(t, got, 1)
	assert.Equal(t, []kv{{"version", "1"}, {"mode", "fast"}, {"level", "info"}, {"body_len", fmt.Sprint(len(body))}, {"resource_name", "pod-u1"}}, got[0].attrs)
	after, err := marshaler.MarshalLogs(ld)
	require.NoError(t, err)
	assert.Equal(t, want, after)
}

// TestConsumeLogsConcurrent runs ConsumeLogs from several goroutines; run
// it with -race. Every pod ends with the facts of its newest record.
func TestConsumeLogsConcurrent(t *testing.T) {
	const (
		goroutines = 8
		batches    = 50
		pods       = 20
	)
	sink := new(consumertest.MetricsSink)
	conn, _ := newTestConnector(t, testConfig(), sink)

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Go(func() {
			for i := range batches {
				ld := plog.NewLogs()
				for p := range pods {
					lrs := addResource(ld, podResource(fmt.Sprintf("u%02d", p)))
					addRecord(lrs, time.Time{}, "noise line")
					// A unique time per goroutine and batch; the version is
					// the time, so the newest record is known.
					tick := i*goroutines + g
					addRecord(lrs, base.Add(time.Duration(tick)*time.Second), fmt.Sprintf("app started version=%d", tick))
				}
				assert.NoError(t, conn.ConsumeLogs(context.Background(), ld))
				_ = conn.state.snapshot()
			}
		})
	}
	wg.Wait()

	snap := conn.state.snapshot().Rules["app_info"]
	require.Len(t, snap, pods)
	newest := fmt.Sprint(batches*goroutines - 1)
	for k, e := range snap {
		assert.Equal(t, newest, e.Facts[0].Value, k)
	}
	// Every emitted series is a valid Info data point.
	for _, s := range allSeries(t, sink.AllMetrics()) {
		assert.Equal(t, int64(1), s.value)
	}
}

func TestConsumeLogsNoMatchDoesNotAllocate(t *testing.T) {
	cfg := loadVLLMRuleFile(t)
	conn, _ := newTestConnector(t, cfg, consumertest.NewNop())
	ld := noMatchLogs(100)
	ld.MarkReadOnly()
	allocs := testing.AllocsPerRun(50, func() {
		_ = conn.ConsumeLogs(context.Background(), ld)
	})
	assert.Zero(t, allocs)
}

func TestNewConnectorCompileError(t *testing.T) {
	cfg := testConfig()
	cfg.Rules[0].Attributes[0].Value = `NoSuchConverter(log.body)`
	conn, err := NewFactory().CreateLogsToMetrics(t.Context(), connectortest.NewNopSettings(component.MustNewType(TypeStr)), cfg, consumertest.NewNop())
	assert.ErrorContains(t, err, "rules[0] (app_info): attributes[0]")
	assert.Nil(t, conn)
}

// countingConsumer counts ConsumeMetrics calls and returns err.
type countingConsumer struct {
	mu  sync.Mutex
	n   int
	err error
}

func (c *countingConsumer) ConsumeMetrics(context.Context, pmetric.Metrics) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	return c.err
}

func (*countingConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (c *countingConsumer) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}
