// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestTruncateUTF8(t *testing.T) {
	testCases := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{name: "shorter", in: "abc", n: 4, want: "abc"},
		{name: "exact", in: "abcd", n: 4, want: "abcd"},
		{name: "ascii", in: "abcdef", n: 4, want: "abcd"},
		{name: "cut_before_two_byte_char", in: "abcé", n: 4, want: "abc"},
		{name: "two_byte_char_fits", in: "abé!", n: 4, want: "abé"},
		{name: "cut_inside_three_byte_char", in: "日本語", n: 4, want: "日"},
		{name: "three_byte_char_fits", in: "日本語", n: 6, want: "日本"},
		{name: "cut_inside_four_byte_char", in: "a😀b", n: 4, want: "a"},
		{name: "four_byte_char_fits", in: "a😀b", n: 5, want: "a😀"},
		{name: "first_char_does_not_fit", in: "é", n: 1, want: ""},
		{name: "empty", in: "", n: 1, want: ""},
		// Invalid UTF-8 has no character start to back up to, so it is cut at n.
		{name: "invalid_utf8", in: "a\x80\x80\x80\x80\x80", n: 5, want: "a\x80\x80\x80\x80"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateUTF8(tc.in, tc.n)
			assert.Equal(t, tc.want, got)
			assert.LessOrEqual(t, len(got), tc.n)
			if utf8.ValidString(tc.in) {
				assert.True(t, utf8.ValidString(got))
			}
		})
	}
}

func TestFitValue(t *testing.T) {
	const r = "\uFFFD"
	testCases := []struct {
		name string
		in   string
		n    int
		want string
		// cut is set when the cut shortens the value after the replacement.
		cut bool
	}{
		{name: "valid", in: "abc", n: 4, want: "abc"},
		{name: "valid_cut", in: "abcdef", n: 4, want: "abcd", cut: true},
		{name: "invalid_byte", in: "a\xffb", n: 8, want: "a" + r + "b"},
		// A run of invalid bytes gives one U+FFFD.
		{name: "invalid_run", in: "a\xff\xfe\xfdb", n: 8, want: "a" + r + "b"},
		{name: "incomplete_sequence", in: "a\xe6\x97", n: 8, want: "a" + r},
		{name: "lone_continuation_byte", in: "\x80", n: 8, want: r},
		// The replacement makes the value longer, 4 bytes to 6. The cut
		// after it keeps n and does not split U+FFFD.
		{name: "replacement_then_cut", in: "abc\x80", n: 4, want: "abc", cut: true},
		{name: "replacement_fits", in: "a\x80", n: 4, want: "a" + r},
		// The replacement makes the value shorter, 5 bytes to 3, so there
		// is no cut.
		{name: "replacement_shorter", in: "\x80\x81\x82\x83\x84", n: 3, want: r},
		{name: "replacement_does_not_fit", in: "\xff", n: 2, want: "", cut: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, fitValue(tc.in, tc.n))
			count := 0
			got := fitCounted(tc.in, tc.n, &count)
			assert.Equal(t, tc.want, got)
			assert.True(t, utf8.ValidString(got))
			assert.LessOrEqual(t, len(got), tc.n)
			assert.Equal(t, tc.cut, count == 1)
		})
	}

	// A valid value is checked without allocating and returned as is.
	valid := strings.Repeat("vLLM 日本 😀 ", 40)
	var got string
	allocs := testing.AllocsPerRun(10, func() {
		got = fitValue(valid, 1024)
	})
	assert.Zero(t, allocs)
	assert.True(t, sameMemory(valid, got))
}

func TestValueString(t *testing.T) {
	m := pcommon.NewMap()
	m.PutStr("a", "b")
	s := pcommon.NewSlice()
	s.AppendEmpty().SetInt(1)
	s.AppendEmpty().SetStr("x")
	testCases := []struct {
		name string
		in   any
		want string
	}{
		{name: "nil", in: nil, want: ""},
		{name: "string", in: "v", want: "v"},
		{name: "empty_string", in: "", want: ""},
		{name: "int64", in: int64(4), want: "4"},
		{name: "int", in: 4, want: "4"},
		{name: "bool", in: true, want: "true"},
		{name: "double", in: 1.5, want: "1.5"},
		{name: "double_integral", in: 2.0, want: "2"},
		{name: "bytes", in: []byte("hi"), want: "aGk="},
		{name: "raw_map", in: map[string]any{"a": "b"}, want: `{"a":"b"}`},
		{name: "raw_slice", in: []any{int64(1), "x"}, want: `[1,"x"]`},
		{name: "pcommon_value", in: pcommon.NewValueInt(7), want: "7"},
		{name: "pcommon_map", in: m, want: `{"a":"b"}`},
		{name: "pcommon_slice", in: s, want: `[1,"x"]`},
		{name: "time_uses_fmt", in: time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC), want: "2026-10-15 12:00:00 +0000 UTC"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, valueString(tc.in))
		})
	}
}

func TestEncodeIdentity(t *testing.T) {
	enc := func(values ...string) string {
		pairs := make([]kv, len(values))
		for i, v := range values {
			pairs[i] = kv{Key: "k", Value: v}
		}
		return encodeIdentity(pairs)
	}
	assert.Equal(t, "36:4b7f0c3e-0000-4000-8000-000000000001", enc("4b7f0c3e-0000-4000-8000-000000000001"))
	assert.Equal(t, "2:ab1:c", enc("ab", "c"))
	// Lists that concatenate to the same text, or that contain the
	// separator, still encode differently.
	distinct := [][]string{
		{"ab", "c"}, {"a", "bc"}, {"abc"},
		{"1:a", "b"}, {"1", "a:b"}, {"1:a:b"},
		{"2:ab"}, {"ab"},
	}
	seen := map[string][]string{}
	for _, values := range distinct {
		key := enc(values...)
		if prev, dup := seen[key]; dup {
			t.Fatalf("%q and %q encode to the same key %q", prev, values, key)
		}
		seen[key] = values
	}
}

func TestCompileRules(t *testing.T) {
	cfg := testConfig()
	cfg.Rules = append(cfg.Rules, RuleConfig{
		Name:                      "cond_info",
		Conditions:                []string{`attributes["event"] == "start"`, `attributes["event"] == "restart"`},
		Identity:                  []string{"k8s.pod.uid"},
		IncludeResourceAttributes: []string{"k8s.pod.uid"},
		Attributes:                []AttributeConfig{{Key: "version"}},
	})
	require.NoError(t, cfg.Validate())
	rules, err := compileRules(cfg, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	require.Len(t, rules, 2)
	assert.Nil(t, rules[0].conditions, "a rule without conditions has no condition sequence")
	assert.NotNil(t, rules[1].conditions)
	require.Len(t, rules[0].attrs, 3)
	assert.Equal(t, "version", rules[0].attrs[0].key)
	assert.Equal(t, "default", rules[0].attrs[1].defaultValue)
	assert.True(t, rules[0].attrs[2].optional)

	// Conditions are ORed.
	for event, want := range map[string]bool{"start": true, "restart": true, "stop": false} {
		lr := plog.NewLogRecord()
		lr.Attributes().PutStr("event", event)
		got, err := rules[1].matchesConditions(t.Context(), newTCtx(lr))
		require.NoError(t, err)
		assert.Equal(t, want, got, event)
	}

	// An unvalidated configuration with a bad expression fails to compile.
	bad := testConfig()
	bad.Rules[0].Attributes[0].Value = `NoSuchConverter(body)`
	_, err = compileRules(bad, componenttest.NewNopTelemetrySettings())
	assert.ErrorContains(t, err, "rules[0] (app_info): attributes[0]")
	bad = testConfig()
	bad.Rules[0].Conditions = []string{`IsMatch(body, `}
	_, err = compileRules(bad, componenttest.NewNopTelemetrySettings())
	assert.ErrorContains(t, err, "rules[0] (app_info): conditions")
}

func TestMatchesBody(t *testing.T) {
	rule := compiledRule{cfg: &RuleConfig{BodyContains: []string{"needle one", "needle two"}}}
	testCases := []struct {
		name string
		set  func(pcommon.Value)
		want bool
	}{
		{name: "first_substring", set: func(v pcommon.Value) { v.SetStr("a needle one b") }, want: true},
		{name: "second_substring", set: func(v pcommon.Value) { v.SetStr("needle two") }, want: true},
		{name: "no_substring", set: func(v pcommon.Value) { v.SetStr("needle three") }},
		{name: "case_sensitive", set: func(v pcommon.Value) { v.SetStr("Needle One") }},
		{name: "empty_string", set: func(v pcommon.Value) { v.SetStr("") }},
		{name: "empty_body", set: func(pcommon.Value) {}},
		{name: "bytes_body", set: func(v pcommon.Value) { v.SetEmptyBytes().FromRaw([]byte("needle one")) }},
		{name: "map_body", set: func(v pcommon.Value) { v.SetEmptyMap().PutStr("message", "needle one") }},
		{name: "slice_body", set: func(v pcommon.Value) { v.SetEmptySlice().AppendEmpty().SetStr("needle one") }},
		{name: "int_body", set: func(v pcommon.Value) { v.SetInt(1) }},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lr := plog.NewLogRecord()
			tc.set(lr.Body())
			assert.Equal(t, tc.want, rule.matchesBody(lr))
		})
	}

	// A rule without body_contains passes every record, whatever its body.
	noBody := compiledRule{cfg: &RuleConfig{}}
	lr := plog.NewLogRecord()
	lr.Body().SetInt(1)
	assert.True(t, noBody.matchesBody(lr))
}

func TestResolveResource(t *testing.T) {
	rule := compiledRule{cfg: &RuleConfig{
		Identity:                  []string{"k8s.pod.uid", "k8s.container.name"},
		IncludeResourceAttributes: []string{"k8s.pod.name", "k8s.namespace.name", "absent", "empty", "int"},
	}}
	attrs := pcommon.NewMap()
	attrs.PutStr("k8s.pod.uid", "uid-1")
	attrs.PutStr("k8s.container.name", "vllm")
	attrs.PutStr("k8s.namespace.name", "team-a")
	attrs.PutStr("k8s.pod.name", "pod-1")
	attrs.PutStr("empty", "")
	attrs.PutInt("int", 3)

	got := rule.resolveResource(attrs, 256)
	assert.True(t, got.resolved)
	assert.True(t, got.ok)
	assert.Equal(t, []kv{{"k8s.pod.uid", "uid-1"}, {"k8s.container.name", "vllm"}}, got.identity)
	assert.Equal(t, "5:uid-14:vllm", got.key)
	// Config order, not resource order; absent and empty values omitted;
	// non-string values with AsString.
	assert.Equal(t, []kv{{"k8s.pod.name", "pod-1"}, {"k8s.namespace.name", "team-a"}, {"int", "3"}}, got.resource)

	// Truncation applies to identity and resource values, and the key is
	// computed from the truncated identity.
	got = rule.resolveResource(attrs, 3)
	assert.True(t, got.ok)
	assert.Equal(t, []kv{{"k8s.pod.uid", "uid"}, {"k8s.container.name", "vll"}}, got.identity)
	assert.Equal(t, "3:uid3:vll", got.key)
	assert.Equal(t, []kv{{"k8s.pod.name", "pod"}, {"k8s.namespace.name", "tea"}, {"int", "3"}}, got.resource)

	for _, tc := range []struct {
		name string
		edit func(pcommon.Map)
	}{
		{name: "missing_identity", edit: func(m pcommon.Map) { m.Remove("k8s.container.name") }},
		{name: "empty_identity", edit: func(m pcommon.Map) { m.PutStr("k8s.pod.uid", "") }},
		{name: "empty_map_identity", edit: func(m pcommon.Map) { m.PutEmpty("k8s.pod.uid") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := pcommon.NewMap()
			attrs.CopyTo(m)
			tc.edit(m)
			got := rule.resolveResource(m, 256)
			assert.True(t, got.resolved)
			assert.False(t, got.ok)
		})
	}

	// An identity value whose first character does not fit is empty after
	// truncation, so the identity is missing.
	m := pcommon.NewMap()
	m.PutStr("k8s.pod.uid", "é")
	m.PutStr("k8s.container.name", "c")
	assert.False(t, rule.resolveResource(m, 1).ok)

	// Invalid UTF-8 in an identity or resource value is replaced with
	// U+FFFD, and the key is computed from the replaced value.
	m = pcommon.NewMap()
	m.PutStr("k8s.pod.uid", "uid-\xff1")
	m.PutStr("k8s.container.name", "vllm")
	m.PutStr("k8s.pod.name", "pod\xfe\xfd1")
	got = rule.resolveResource(m, 256)
	require.True(t, got.ok)
	assert.Equal(t, []kv{{"k8s.pod.uid", "uid-\uFFFD1"}, {"k8s.container.name", "vllm"}}, got.identity)
	assert.Equal(t, "8:uid-\uFFFD14:vllm", got.key)
	assert.Equal(t, []kv{{"k8s.pod.name", "pod\uFFFD1"}}, got.resource)
	assert.Zero(t, got.truncated)
	// The cut comes after the replacement. "uid-\xff1" is 6 bytes and
	// "uid-\uFFFD1" is 8, so a limit of 6 cuts it before U+FFFD.
	got = rule.resolveResource(m, 6)
	require.True(t, got.ok)
	assert.Equal(t, []kv{{"k8s.pod.uid", "uid-"}, {"k8s.container.name", "vllm"}}, got.identity)
	assert.Equal(t, []kv{{"k8s.pod.name", "pod\uFFFD"}}, got.resource)
	assert.Equal(t, 2, got.truncated)
}

// TestResolvedValuesDoNotAliasInput checks that stored values are copies, so
// the state does not keep a log body or resource alive.
func TestResolvedValuesDoNotAliasInput(t *testing.T) {
	body := "app started version=" + strings.Repeat("9", 10) + " tail"
	cfg := testConfig()
	require.NoError(t, cfg.Validate())
	rules, err := compileRules(cfg, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	lr := plog.NewLogRecord()
	lr.Body().SetStr(body)
	f, err := rules[0].extractFacts(t.Context(), newTCtx(lr), 256, cfg.ErrorMode)
	require.NoError(t, err)
	require.Empty(t, f.missing)
	require.Equal(t, "9999999999", f.values[0].Value)
	assert.False(t, sameMemory(body, f.values[0].Value), "the fact must not point into the body")
}

// TestAppendIdentityKey checks that the allocation-free key of the activity
// path is the key that resolveResource stores.
func TestAppendIdentityKey(t *testing.T) {
	long := strings.Repeat("u", 150)
	for _, tc := range []struct {
		name     string
		identity []string
		attrs    func(pcommon.Map)
		maxLen   int
		wantOK   bool
	}{
		{name: "one_key", identity: []string{"k8s.pod.uid"}, attrs: func(m pcommon.Map) { m.PutStr("k8s.pod.uid", "uid-1") }, maxLen: 256, wantOK: true},
		{name: "two_keys", identity: []string{"k8s.pod.uid", "k8s.container.name"}, attrs: func(m pcommon.Map) {
			m.PutStr("k8s.pod.uid", "uid-1")
			m.PutStr("k8s.container.name", "vllm")
		}, maxLen: 256, wantOK: true},
		{name: "long_value", identity: []string{"k8s.pod.uid"}, attrs: func(m pcommon.Map) { m.PutStr("k8s.pod.uid", long) }, maxLen: 1024, wantOK: true},
		{name: "truncated", identity: []string{"k8s.pod.uid"}, attrs: func(m pcommon.Map) { m.PutStr("k8s.pod.uid", "uid-éé") }, maxLen: 5, wantOK: true},
		{name: "int_value", identity: []string{"id"}, attrs: func(m pcommon.Map) { m.PutInt("id", 42) }, maxLen: 256, wantOK: true},
		{name: "missing", identity: []string{"k8s.pod.uid", "k8s.container.name"}, attrs: func(m pcommon.Map) { m.PutStr("k8s.pod.uid", "uid-1") }, maxLen: 256},
		{name: "empty", identity: []string{"k8s.pod.uid"}, attrs: func(m pcommon.Map) { m.PutStr("k8s.pod.uid", "") }, maxLen: 256},
		{name: "empty_after_truncation", identity: []string{"k8s.pod.uid"}, attrs: func(m pcommon.Map) { m.PutStr("k8s.pod.uid", "é") }, maxLen: 1},
		{name: "invalid_utf8", identity: []string{"k8s.pod.uid", "k8s.container.name"}, attrs: func(m pcommon.Map) {
			m.PutStr("k8s.pod.uid", "uid-\xff\xfe1")
			m.PutStr("k8s.container.name", "v\x80")
		}, maxLen: 256, wantOK: true},
		{name: "invalid_utf8_then_cut", identity: []string{"k8s.pod.uid"}, attrs: func(m pcommon.Map) { m.PutStr("k8s.pod.uid", "uid\x80") }, maxLen: 4, wantOK: true},
		{name: "invalid_utf8_empty_after_cut", identity: []string{"k8s.pod.uid"}, attrs: func(m pcommon.Map) { m.PutStr("k8s.pod.uid", "\x80") }, maxLen: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rule := compiledRule{cfg: &RuleConfig{Identity: tc.identity}}
			attrs := pcommon.NewMap()
			tc.attrs(attrs)
			prefix := []byte("previous")
			got, ok := rule.appendIdentityKey(prefix, attrs, tc.maxLen)
			res := rule.resolveResource(attrs, tc.maxLen)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, res.ok, ok)
			if ok {
				assert.Equal(t, "previous"+res.key, string(got))
			}
		})
	}

	// The key of string values is built without allocating.
	rule := compiledRule{cfg: &RuleConfig{Identity: []string{"k8s.pod.uid", "k8s.container.name"}}}
	attrs := pcommon.NewMap()
	attrs.PutStr("k8s.pod.uid", long)
	attrs.PutStr("k8s.container.name", "vllm")
	buf := make([]byte, 0, 512)
	allocs := testing.AllocsPerRun(10, func() {
		buf, _ = rule.appendIdentityKey(buf[:0], attrs, 256)
	})
	assert.Zero(t, allocs)
}

func TestTruncationCounts(t *testing.T) {
	rule := compiledRule{cfg: &RuleConfig{
		Identity:                  []string{"k8s.pod.uid"},
		IncludeResourceAttributes: []string{"k8s.pod.name", "k8s.namespace.name"},
	}}
	attrs := pcommon.NewMap()
	attrs.PutStr("k8s.pod.uid", "uid-1")
	attrs.PutStr("k8s.pod.name", "pod-1")
	attrs.PutStr("k8s.namespace.name", "ns")
	assert.Equal(t, 0, rule.resolveResource(attrs, 5).truncated)
	assert.Equal(t, 2, rule.resolveResource(attrs, 4).truncated)
	// A missing identity after a cut still counts the cut.
	attrs.PutStr("k8s.pod.uid", "é")
	res := rule.resolveResource(attrs, 1)
	assert.False(t, res.ok)
	assert.Equal(t, 1, res.truncated)

	cfg := testConfig()
	cfg.MaxValueLength = 3
	cfg.Rules[0].Attributes[1].DefaultValue = "d"
	require.NoError(t, cfg.Validate())
	rules, err := compileRules(cfg, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	lr := plog.NewLogRecord()
	lr.Body().SetStr("app started version=1.2.3 mode=fast region=eu")
	f, err := rules[0].extractFacts(t.Context(), newTCtx(lr), cfg.MaxValueLength, cfg.ErrorMode)
	require.NoError(t, err)
	assert.Equal(t, []kv{{"version", "1.2"}, {"mode", "fas"}, {"region", "eu"}}, f.values)
	assert.Equal(t, 2, f.truncated)

	// A required fact without a value names the attribute.
	lr.Body().SetStr("app started mode=fast")
	f, err = rules[0].extractFacts(t.Context(), newTCtx(lr), cfg.MaxValueLength, cfg.ErrorMode)
	require.NoError(t, err)
	assert.Equal(t, "version", f.missing)
	assert.ErrorContains(t, f.missingErr, "key not found in map")
	assert.Nil(t, f.values)
}
