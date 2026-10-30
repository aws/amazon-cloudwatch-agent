// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
)

// obs returns an observation of rule for pod at time t with a record of n
// bytes.
func obs(rule int, pod string, t int64, n int) observation {
	return observation{
		rule:     rule,
		key:      podKey(pod),
		identity: []kv{{"k8s.namespace.name", "ns"}, {"k8s.pod.name", pod}, {"k8s.container.name", "app"}},
		record:   []byte(`{"r":"` + strings.Repeat("x", n) + `"}`),
		time:     t,
	}
}

func twoRuleSpecs(maxEntries int) []ruleSpec {
	return []ruleSpec{
		{name: "a", identity: testIdentity, maxEntries: maxEntries},
		{name: "b", identity: testIdentity, maxEntries: maxEntries},
	}
}

func TestStateMaxEntriesEvictsOldest(t *testing.T) {
	s := newState(twoRuleSpecs(2), maxMaxStateBytes)
	res := s.applyBatch([]observation{obs(0, "p2", 20, 10), obs(0, "p1", 10, 10), obs(1, "p0", 1, 10)})
	assert.Equal(t, 3, res.counts[outcomeInserted])
	res = s.applyBatch([]observation{obs(0, "p3", 30, 10)})
	assert.Equal(t, []removal{{entryRef: entryRef{rule: 0, key: podKey("p1")}, reason: removeMaxEntries}}, res.removed)
	assert.ElementsMatch(t, []string{podKey("p2"), podKey("p3")}, keys(s, 0))
	assert.Equal(t, []string{podKey("p0")}, keys(s, 1), "max_entries is per rule")
}

func TestStateMaxStateBytesEvictsOldestOfAnyRule(t *testing.T) {
	one := entrySize(podKey("p1"), &entry{Identity: obs(0, "p1", 0, 0).identity, Record: obs(0, "p1", 0, 1000).record})
	s := newState(twoRuleSpecs(100), 3*one)
	s.applyBatch([]observation{obs(1, "p1", 10, 1000), obs(0, "p2", 20, 1000), obs(0, "p3", 30, 1000)})
	res := s.applyBatch([]observation{obs(0, "p4", 5, 1000)})
	assert.Equal(t, []removal{{entryRef: entryRef{rule: 1, key: podKey("p1")}, reason: removeMaxStateBytes}}, res.removed,
		"the oldest entry of any rule goes, never the new one, even if it is older")
	assert.ElementsMatch(t, []string{podKey("p2"), podKey("p3"), podKey("p4")}, keys(s, 0))
	counts, bytes := s.sizes()
	assert.LessOrEqual(t, bytes, 3*one)
	assert.Equal(t, []int{3, 0}, counts)
}

func TestStateTooLarge(t *testing.T) {
	s := newState(twoRuleSpecs(100), minMaxStateBytes)
	res := s.applyBatch([]observation{obs(0, "p1", 10, minMaxStateBytes)})
	assert.Equal(t, 1, res.counts[outcomeTooLarge])
	assert.Equal(t, []removal{{entryRef: entryRef{rule: 0, key: podKey("p1")}, reason: removeTooLarge}}, res.rejected)
	assert.Empty(t, s.entries())
	assert.False(t, res.changed(), "nothing was stored or removed")

	// A record that makes a kept entry too large removes it, because the
	// kept record is stale.
	s.applyBatch([]observation{obs(0, "p1", 10, 10)})
	res = s.applyBatch([]observation{obs(0, "p1", 20, minMaxStateBytes)})
	assert.Equal(t, []removal{{entryRef: entryRef{rule: 0, key: podKey("p1")}, reason: removeTooLarge}}, res.removed)
	assert.Empty(t, res.rejected)
	assert.Empty(t, s.entries())
	_, bytes := s.sizes()
	assert.Zero(t, bytes)
}

func TestStateOutcomes(t *testing.T) {
	s := newState(twoRuleSpecs(100), maxMaxStateBytes)
	res := s.applyBatch([]observation{obs(0, "p1", 10, 1)})
	assert.True(t, res.changed())
	res = s.applyBatch([]observation{obs(0, "p1", 10, 1)})
	assert.Equal(t, 1, res.counts[outcomeUnchanged])
	assert.False(t, res.changed())
	res = s.applyBatch([]observation{obs(0, "p1", 9, 1)})
	assert.Equal(t, 1, res.counts[outcomeIgnoredOlder])
	assert.False(t, res.changed())
	// The same time with another record or source file replaces it.
	res = s.applyBatch([]observation{obs(0, "p1", 10, 2)})
	assert.Equal(t, 1, res.counts[outcomeReplaced])
	o := obs(0, "p1", 10, 2)
	o.sourceFile = "/other"
	res = s.applyBatch([]observation{o})
	assert.Equal(t, 1, res.counts[outcomeReplaced])
	assert.True(t, res.changed())
	_, bytes := s.sizes()
	assert.Equal(t, entrySize(podKey("p1"), &s.entries()[0].entry), bytes)
}

// TestEntrySizeCoversEncoding checks that the size estimate of each entry
// is at least its encoded size in a snapshot, with the comma after it, also
// for keys, values, and records that encoding/json escapes.
func TestEntrySizeCoversEncoding(t *testing.T) {
	odd := "a\"b\\c\n<&>\u2028\u2029\x01\xff\u00e9"
	ld := plog.NewLogs()
	res := podResource("p<1>")
	res["k8s.pod.label.note"] = odd
	addRecord(addResource(ld, res), base, "app started "+odd, podFile("p&1"))
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
	kept := conn.state.entries()
	require.Len(t, kept, 1)

	for _, e := range []struct {
		key   string
		entry entry
	}{
		{kept[0].key, kept[0].entry},
		{odd, entry{Identity: []kv{{odd, odd}}, Record: json.RawMessage(`{}`), SourceFile: odd, Time: -1 << 63}},
		{"", entry{Record: json.RawMessage(`{}`)}},
	} {
		b, err := json.Marshal(map[string]entry{e.key: e.entry})
		require.NoError(t, err)
		// The map braces stand for the comma that separates entries.
		assert.LessOrEqual(t, len(b)-1, entrySize(e.key, &e.entry), e.key)
	}
}

// TestEncodeRecordIsEncodedForm checks that encoding/json writes a kept
// record verbatim, so that its length is its encoded length.
func TestEncodeRecordIsEncodedForm(t *testing.T) {
	ld := podLogs("p1", base, "app started <a> & \u2028 \u2029")
	rl := ld.ResourceLogs().At(0)
	sl := rl.ScopeLogs().At(0)
	data, err := encodeRecord(rl, sl, sl.LogRecords().At(0))
	require.NoError(t, err)
	b, err := json.Marshal(entry{Record: data})
	require.NoError(t, err)
	assert.Contains(t, string(b), `"record":`+string(data)+`,`)
	assert.NotContains(t, string(data), "<")
	assert.NotContains(t, string(data), "\u2028")

	got, err := decodeRecord(data)
	require.NoError(t, err)
	assert.Equal(t, "app started <a> & \u2028 \u2029", got.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str())
}

func TestDecodeRecordRequiresOneRecord(t *testing.T) {
	two := mergeLogs(podLogs("p1", base, "a"), podLogs("p2", base, "b"))
	b, err := jsonMarshaler.MarshalLogs(two)
	require.NoError(t, err)
	_, err = decodeRecord(b)
	assert.ErrorContains(t, err, "exactly one log record, got 2")
	b, err = jsonMarshaler.MarshalLogs(plog.NewLogs())
	require.NoError(t, err)
	_, err = decodeRecord(b)
	assert.Error(t, err)
	_, err = decodeRecord([]byte("{"))
	assert.Error(t, err)
}

func TestJSONStringLen(t *testing.T) {
	for _, s := range []string{"", "plain", "a\"b", "\\", "\n\r\t\b\f", "\x00\x1f", "<>&", "\u00e9\u20ac\U0001f600", "\u2028\u2029", "\xff\xfe", "a\xffb"} {
		b, err := json.Marshal(s)
		require.NoError(t, err)
		assert.Equal(t, len(b)-2, jsonStringLen(s), "%q", s)
	}
}

func TestStateRestoreKeepsMostRecent(t *testing.T) {
	s := newState(twoRuleSpecs(2), maxMaxStateBytes)
	s.applyBatch([]observation{obs(0, "old", 1, 1)})
	items := []restoredEntry{}
	for _, o := range []observation{obs(0, "p1", 30, 1), obs(0, "p2", 10, 1), obs(0, "p3", 20, 1), obs(1, "p4", 5, 1)} {
		items = append(items, restoredEntry{
			entryRef: entryRef{rule: o.rule, key: o.key},
			entry:    entry{Identity: o.identity, Record: o.record, Time: o.time},
		})
	}
	kept, dropped := s.restore(items, 42)
	assert.Equal(t, 3, kept)
	assert.Equal(t, 1, dropped)
	assert.ElementsMatch(t, []string{podKey("p1"), podKey("p3")}, keys(s, 0), "the restore replaces the state and keeps the most recent")
	assert.Equal(t, []string{podKey("p4")}, keys(s, 1))
}

func TestStateSnapshot(t *testing.T) {
	s := newState(twoRuleSpecs(10), maxMaxStateBytes)
	assert.Empty(t, s.snapshot().Rules)
	s.applyBatch([]observation{obs(1, "p1", 10, 1)})
	snap := s.snapshot()
	require.Len(t, snap.Rules, 1)
	assert.Equal(t, int64(10), snap.Rules["b"][podKey("p1")].Time)
}

// TestStateMaxEntriesRejectsOlder checks that a full rule does not evict a
// newer record for an older one, because the rule keeps the latest records.
func TestStateMaxEntriesRejectsOlder(t *testing.T) {
	s := newState(twoRuleSpecs(2), maxMaxStateBytes)
	s.applyBatch([]observation{obs(0, "p1", 10, 1), obs(0, "p2", 20, 1)})
	res := s.applyBatch([]observation{obs(0, "p0", 5, 1)})
	assert.Equal(t, 1, res.counts[outcomeOverLimit])
	assert.Empty(t, res.removed)
	assert.Equal(t, []removal{{entryRef: entryRef{rule: 0, key: podKey("p0")}, reason: removeMaxEntries}}, res.rejected)
	assert.False(t, res.changed())
	assert.ElementsMatch(t, []string{podKey("p1"), podKey("p2")}, keys(s, 0))

	// A record as old as the oldest kept one evicts it.
	res = s.applyBatch([]observation{obs(0, "p3", 10, 1)})
	assert.Equal(t, []removal{{entryRef: entryRef{rule: 0, key: podKey("p1")}, reason: removeMaxEntries}}, res.removed)
	assert.ElementsMatch(t, []string{podKey("p2"), podKey("p3")}, keys(s, 0))
}

func TestStateTooLargeObservation(t *testing.T) {
	s := newState(twoRuleSpecs(10), maxMaxStateBytes)
	s.applyBatch([]observation{obs(0, "p1", 10, 1)})
	big := func(t int64) observation {
		return observation{rule: 0, key: podKey("p1"), time: t, tooLarge: true}
	}
	// An older record that is too large leaves the newer kept one.
	res := s.applyBatch([]observation{big(9)})
	assert.Equal(t, 1, res.counts[outcomeTooLarge])
	assert.Empty(t, res.removed)
	assert.Len(t, s.entries(), 1)
	// A newer one removes it, because it is stale.
	res = s.applyBatch([]observation{big(11)})
	assert.Equal(t, []removal{{entryRef: entryRef{rule: 0, key: podKey("p1")}, reason: removeTooLarge}}, res.removed)
	assert.Empty(t, s.entries())
	assert.True(t, res.changed())
	// With no kept record, nothing changes.
	res = s.applyBatch([]observation{big(12)})
	assert.Empty(t, res.removed)
	assert.Empty(t, res.rejected)
	assert.False(t, res.changed())
}

func TestStateLastReplay(t *testing.T) {
	s := newState(twoRuleSpecs(10), maxMaxStateBytes)
	assert.Zero(t, s.lastReplayTime())
	assert.Zero(t, s.snapshot().LastReplay)
	s.setLastReplay(7)
	assert.Equal(t, int64(7), s.snapshot().LastReplay)
	s.restore(nil, 3)
	assert.Equal(t, int64(3), s.lastReplayTime())
}
