// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"bytes"
	"io/fs"
	"maps"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

var testNow = time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)

var base = time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)

var testIdentity = []string{"k8s.namespace.name", "k8s.pod.name", "k8s.container.name"}

// testConfig keeps the "app started" line of each container.
func testConfig() *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.SourceFileAttribute = "log.file.path"
	cfg.Rules = []RuleConfig{{
		Name:         "app_start",
		BodyContains: []string{"app started"},
		Identity:     testIdentity,
	}}
	return cfg
}

// newTestConnector uses the clock testNow, an observed debug logger, no jitter, and source files
// that all exist.
func newTestConnector(t testing.TB, cfg *Config, next consumer.Logs) (*replayConnector, *observer.ObservedLogs) {
	t.Helper()
	require.NoError(t, cfg.Validate())
	core, logs := observer.New(zapcore.DebugLevel)
	set := connectortest.NewNopSettings(component.MustNewType(TypeStr))
	set.Logger = zap.New(core)
	conn, err := newConnector(set, cfg, next)
	require.NoError(t, err)
	conn.now = func() time.Time { return testNow }
	conn.checkSource = func(string) error { return nil }
	conn.recheckDelay = 0
	conn.jitter = func(time.Duration) time.Duration { return 0 }
	return conn, logs
}

// fakeFiles answers checkSource. A path is present unless it was removed or given an error.
type fakeFiles struct {
	mu      sync.Mutex
	errs    map[string]error
	checked map[string]int
}

func newFakeFiles() *fakeFiles {
	return &fakeFiles{errs: map[string]error{}, checked: map[string]int{}}
}

func (f *fakeFiles) stat(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checked[path]++
	return f.errs[path]
}

func (f *fakeFiles) remove(path string) {
	f.setErr(path, &fs.PathError{Op: "stat", Path: path, Err: fs.ErrNotExist})
}

func (f *fakeFiles) setErr(path string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs[path] = err
}

func (f *fakeFiles) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.checked[path]
}

// podResource also sets attributes that the identity does not use.
func podResource(pod string) map[string]string {
	return map[string]string{
		"k8s.namespace.name": "ns",
		"k8s.pod.name":       pod,
		"k8s.container.name": "app",
		"k8s.pod.uid":        "uid-" + pod,
		"k8s.pod.label.app":  "demo",
	}
}

func podFile(pod string) string {
	return "/var/log/containers/" + pod + "_ns_app-0123456789abcdef.log"
}

func podKey(pod string) string {
	return encodeIdentity([]kv{{"k8s.namespace.name", "ns"}, {"k8s.pod.name", pod}, {"k8s.container.name", "app"}})
}

func addResource(ld plog.Logs, attrs map[string]string) plog.ScopeLogs {
	rl := ld.ResourceLogs().AppendEmpty()
	rl.SetSchemaUrl("https://opentelemetry.io/schemas/1.38.0")
	// Sorted, so that two calls with the same attributes give the same record.
	for _, k := range slices.Sorted(maps.Keys(attrs)) {
		rl.Resource().Attributes().PutStr(k, attrs[k])
	}
	sl := rl.ScopeLogs().AppendEmpty()
	sl.SetSchemaUrl("https://example.com/scope-schema")
	sl.Scope().SetName("app-logs")
	sl.Scope().SetVersion("1.0")
	sl.Scope().Attributes().PutStr("cloudwatch.pipeline", "application-logs")
	return sl
}

// addRecord sets an observed time one second after ts. A zero ts leaves both times unset, and an
// empty path leaves log.file.path unset.
func addRecord(sl plog.ScopeLogs, ts time.Time, body, path string) plog.LogRecord {
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr(body)
	if !ts.IsZero() {
		lr.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(ts.Add(time.Second)))
	}
	lr.SetSeverityNumber(plog.SeverityNumberInfo)
	lr.SetSeverityText("INFO")
	lr.Attributes().PutStr("log.iostream", "stdout")
	lr.Attributes().PutStr("logtag", "F")
	if path != "" {
		lr.Attributes().PutStr("log.file.path", path)
	}
	return lr
}

func podLogs(pod string, ts time.Time, bodies ...string) plog.Logs {
	ld := plog.NewLogs()
	sl := addResource(ld, podResource(pod))
	for _, body := range bodies {
		addRecord(sl, ts, body, podFile(pod))
	}
	return ld
}

// copyOf returns the copy that the connector sends at ts for record k of ResourceLogs i.
func copyOf(ld plog.Logs, i, k int, ts time.Time) plog.Logs {
	rl := ld.ResourceLogs().At(i)
	sl := rl.ScopeLogs().At(0)
	out := plog.NewLogs()
	orl := out.ResourceLogs().AppendEmpty()
	rl.Resource().CopyTo(orl.Resource())
	orl.SetSchemaUrl(rl.SchemaUrl())
	osl := orl.ScopeLogs().AppendEmpty()
	sl.Scope().CopyTo(osl.Scope())
	osl.SetSchemaUrl(sl.SchemaUrl())
	lr := osl.LogRecords().AppendEmpty()
	sl.LogRecords().At(k).CopyTo(lr)
	lr.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(ts))
	return out
}

func mergeLogs(lds ...plog.Logs) plog.Logs {
	out := plog.NewLogs()
	for _, ld := range lds {
		c := plog.NewLogs()
		ld.CopyTo(c)
		c.ResourceLogs().MoveAndAppendTo(out.ResourceLogs())
	}
	return out
}

// requireLogsEqual compares as OTLP protobuf, which also sees values that JSON cannot show.
func requireLogsEqual(t testing.TB, want, got plog.Logs) {
	t.Helper()
	var proto plog.ProtoMarshaler
	w, err := proto.MarshalLogs(want)
	require.NoError(t, err)
	g, err := proto.MarshalLogs(got)
	require.NoError(t, err)
	if bytes.Equal(w, g) {
		return
	}
	wj, err := jsonMarshaler.MarshalLogs(want)
	require.NoError(t, err)
	gj, err := jsonMarshaler.MarshalLogs(got)
	require.NoError(t, err)
	require.JSONEq(t, string(wj), string(gj))
	require.Fail(t, "the logs differ as protobuf but not as JSON")
}

func keys(s *state, rule int) []string {
	var out []string
	for _, k := range s.entries() {
		if k.rule == rule {
			out = append(out, k.key)
		}
	}
	return out
}
