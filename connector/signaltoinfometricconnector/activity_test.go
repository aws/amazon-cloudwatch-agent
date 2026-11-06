// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// hookCounts counts the calls of the state hooks of a connector.
type hookCounts struct {
	changed, touched int
}

func countHooks(conn *infoMetricConnector) *hookCounts {
	h := &hookCounts{}
	conn.hooks = stateHooks{
		changed: func() { h.changed++ },
		touched: func() { h.touched++ },
	}
	return h
}

// activityOf returns the Activity of the app_info entity of a pod.
func activityOf(t *testing.T, conn *infoMetricConnector, uid string) time.Time {
	t.Helper()
	return ruleActivityOf(t, conn, "app_info", key(uid))
}

// ruleActivityOf returns the Activity of the entity of a rule with the
// state key k.
func ruleActivityOf(t *testing.T, conn *infoMetricConnector, rule, k string) time.Time {
	t.Helper()
	e, ok := conn.state.snapshot().Rules[rule][k]
	require.True(t, ok, k)
	return time.Unix(0, e.Activity).UTC()
}

func TestActivityFromNonMatchingRecordsDelaysEviction(t *testing.T) {
	t1, t2, t3, t4 := base, base.Add(time.Minute), base.Add(2*time.Minute), base.Add(3*time.Minute)
	for _, tc := range []struct {
		name string
		// consume sends the u1 noise at t3 and the u3 startup line at t4.
		consume func(*testing.T, *infoMetricConnector)
	}{
		{name: "separate_batches", consume: func(t *testing.T, conn *infoMetricConnector) {
			require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", t3, "INFO request handled", "noise")))
			require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u3", t4, "app started version=1")))
		}},
		{name: "same_batch", consume: func(t *testing.T, conn *infoMetricConnector) {
			// The activity of the batch is applied before its observations.
			ld := podLogs("u3", t4, "app started version=1")
			addRecord(addResource(ld, podResource("u1")), t3, "noise")
			require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.Rules[0].MaxEntities = 2
			conn, _ := newTestConnector(t, cfg, consumertest.NewNop())
			require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", t1, "app started version=1")))
			require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u2", t2, "app started version=1")))

			// u1 keeps logging lines that the rule does not match, and u2
			// goes quiet: u2 is evicted, not u1.
			tc.consume(t, conn)
			snap := conn.state.snapshot().Rules["app_info"]
			assert.Len(t, snap, 2)
			assert.Contains(t, snap, key("u1"))
			assert.Contains(t, snap, key("u3"))
			assert.NotContains(t, snap, key("u2"))
			assert.Equal(t, t3, activityOf(t, conn, "u1"))
			// The facts and their times stay.
			assert.Equal(t, t1.UnixNano(), snap[key("u1")].Updated)
			assert.Equal(t, t1.UnixNano(), snap[key("u1")].FirstSeen)
		})
	}
}

func TestActivityLatestRecordOfResource(t *testing.T) {
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	hooks := countHooks(conn)
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "app started version=1")))
	assert.Equal(t, hookCounts{changed: 1}, *hooks)

	// Records out of order, over two ScopeLogs, one with only an observed
	// timestamp.
	ld := plog.NewLogs()
	lrs := addResource(ld, podResource("u1"))
	addRecord(lrs, base.Add(3*time.Second), "noise")
	addRecord(lrs, base.Add(5*time.Second), "noise")
	addRecord(lrs, base.Add(4*time.Second), "noise")
	lr := ld.ResourceLogs().At(0).ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(base.Add(2 * time.Second)))
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
	assert.Equal(t, base.Add(5*time.Second), activityOf(t, conn, "u1"))
	assert.Equal(t, hookCounts{changed: 1, touched: 1}, *hooks)

	// A later observed timestamp counts too.
	ld = plog.NewLogs()
	lr = addResource(ld, podResource("u1")).AppendEmpty()
	lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(base.Add(6 * time.Second)))
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))
	assert.Equal(t, base.Add(6*time.Second), activityOf(t, conn, "u1"))
}

func TestActivityUntrackedPodCreatesNoEntity(t *testing.T) {
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "app started version=1")))
	hooks := countHooks(conn)
	before := conn.state.snapshot()

	ld := podLogs("u9", base.Add(time.Hour), "noise", "INFO request handled")
	// A resource without the identity, and one without records.
	addRecord(addResource(ld, map[string]string{"k8s.pod.name": "no-uid"}), base.Add(time.Hour), "noise")
	addResource(ld, podResource("u1"))
	require.NoError(t, conn.ConsumeLogs(t.Context(), ld))

	assert.Equal(t, before, conn.state.snapshot())
	assert.Equal(t, hookCounts{}, *hooks)
}

// TestActivityInvalidUTF8Identity checks that the activity path builds the
// state key that the entity was stored with when the identity value has
// invalid UTF-8, so that a later record of the pod still moves its
// Activity.
func TestActivityInvalidUTF8Identity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		maxLen int
		uid    string
		// stored is the identity value of the state key.
		stored string
	}{
		{name: "replaced", maxLen: 256, uid: "u\xff\xfe1", stored: "u\uFFFD1"},
		// "abc\x80" fits 4 bytes, but "abc\uFFFD" does not.
		{name: "replaced_then_cut", maxLen: 4, uid: "abc\x80", stored: "abc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.MaxValueLength = tc.maxLen
			cfg.Rules[0].Attributes[1].DefaultValue = "d"
			conn, _ := newTestConnector(t, cfg, consumertest.NewNop())
			require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs(tc.uid, base, "app started version=1")))
			assert.Equal(t, base, activityOf(t, conn, tc.stored))
			hooks := countHooks(conn)
			require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs(tc.uid, base.Add(time.Minute), "noise")))
			assert.Equal(t, base.Add(time.Minute), activityOf(t, conn, tc.stored))
			assert.Equal(t, hookCounts{touched: 1}, *hooks)
		})
	}
}

func TestActivityClampsFutureTime(t *testing.T) {
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", base, "app started version=1")))

	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", testNow.Add(time.Hour), "noise")))
	assert.Equal(t, testNow, activityOf(t, conn, "u1"), "a record time ahead of now is now")

	// A record without any timestamp is now.
	conn.now = func() time.Time { return testNow.Add(time.Minute) }
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", time.Time{}, "noise")))
	assert.Equal(t, testNow.Add(time.Minute), activityOf(t, conn, "u1"))

	// The time of a matching record is clamped too, so that it cannot pin
	// the entity either.
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u2", testNow.Add(24*time.Hour), "app started version=1")))
	e := conn.state.snapshot().Rules["app_info"][key("u2")]
	now := testNow.Add(time.Minute).UnixNano()
	assert.Equal(t, entity{
		Identity:  e.Identity,
		Resource:  e.Resource,
		Facts:     e.Facts,
		FirstSeen: now,
		Updated:   now,
		Activity:  now,
	}, e)
}

func TestActivityNeverMovesBackward(t *testing.T) {
	conn, _ := newTestConnector(t, testConfig(), consumertest.NewNop())
	t1, t2, t3 := base, base.Add(time.Minute), base.Add(2*time.Minute)
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", t2, "app started version=1")))
	hooks := countHooks(conn)

	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", t1, "noise")))
	assert.Equal(t, t2, activityOf(t, conn, "u1"))
	assert.Equal(t, hookCounts{}, *hooks, "nothing moved")

	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", t3, "noise")))
	assert.Equal(t, t3, activityOf(t, conn, "u1"))
	assert.Equal(t, hookCounts{touched: 1}, *hooks)

	// The same records again, and an older matching record with the same
	// facts: no movement.
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", t3, "noise")))
	require.NoError(t, conn.ConsumeLogs(t.Context(), podLogs("u1", t1, "app started version=1")))
	assert.Equal(t, t3, activityOf(t, conn, "u1"))
	assert.Equal(t, hookCounts{touched: 1}, *hooks)
}

func TestStateHooks(t *testing.T) {
	cfg := testConfig()
	cfg.Rules[0].MaxEntities = 1
	conn, _ := newTestConnector(t, cfg, consumertest.NewNop())
	hooks := countHooks(conn)
	steps := []struct {
		name string
		ld   plog.Logs
		want hookCounts
	}{
		{name: "noise_without_entities", ld: podLogs("u1", base, "noise"), want: hookCounts{}},
		{name: "insert", ld: podLogs("u1", base, "app started version=1"), want: hookCounts{changed: 1}},
		{name: "replay", ld: podLogs("u1", base, "app started version=1"), want: hookCounts{changed: 1}},
		{name: "same_facts_later", ld: podLogs("u1", base.Add(time.Second), "app started version=1"), want: hookCounts{changed: 1, touched: 1}},
		// The record also moves the activity, so the batch calls both.
		{name: "change", ld: podLogs("u1", base.Add(2*time.Second), "app started version=2"), want: hookCounts{changed: 2, touched: 2}},
		{name: "older_facts", ld: podLogs("u1", base, "app started version=0"), want: hookCounts{changed: 2, touched: 2}},
		{name: "insert_and_evict", ld: podLogs("u2", base.Add(3*time.Second), "app started version=1"), want: hookCounts{changed: 3, touched: 2}},
		{name: "evicted_pod_noise", ld: podLogs("u1", base.Add(4*time.Second), "noise"), want: hookCounts{changed: 3, touched: 2}},
		{name: "activity", ld: podLogs("u2", base.Add(4*time.Second), "noise"), want: hookCounts{changed: 3, touched: 3}},
	}
	for _, st := range steps {
		require.NoError(t, conn.ConsumeLogs(t.Context(), st.ld), st.name)
		assert.Equal(t, st.want, *hooks, st.name)
	}
}

func TestConsumeLogsTrackedActivityDoesNotAllocate(t *testing.T) {
	if raceEnabled {
		t.Skip("sync.Pool drops items at random under the race detector")
	}
	conn := newTrackedConnector(t, 20)
	ld := trackedActivityLogs(20, 50)
	ld.MarkReadOnly()
	allocs := testing.AllocsPerRun(50, func() {
		_ = conn.ConsumeLogs(context.Background(), ld)
	})
	assert.Zero(t, allocs)
	assert.Equal(t, base.Add(49*time.Millisecond), ruleActivityOf(t, conn, "vllm_engine_config_info", vllmKey("vllm-07")))
}
