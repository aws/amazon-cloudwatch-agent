// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test rules for the state: rule 0 "a" and rule 1 "b", both keyed by k8s.pod.uid.
func testSpecs(maxEntities int) []ruleSpec {
	return []ruleSpec{
		{name: "a", identity: []string{"k8s.pod.uid"}, maxEntities: maxEntities},
		{name: "b", identity: []string{"k8s.pod.uid"}, maxEntities: maxEntities},
	}
}

// obs returns an observation for rule and pod uid with the given version
// fact at time t.
func obs(rule int, uid, version string, t int64) observation {
	identity := []kv{{Key: "k8s.pod.uid", Value: uid}}
	return observation{
		rule:     rule,
		key:      encodeIdentity(identity),
		identity: identity,
		resource: []kv{{Key: "k8s.pod.name", Value: "pod-" + uid}},
		facts:    []kv{{Key: "version", Value: version}},
		time:     t,
	}
}

func key(uid string) string {
	return encodeIdentity([]kv{{Key: "k8s.pod.uid", Value: uid}})
}

func ref(rule int, uid string) entityRef {
	return entityRef{rule: rule, key: key(uid)}
}

// refs returns the entities of evictions, without the reasons.
func refs(evicted []eviction) []entityRef {
	if evicted == nil {
		return nil
	}
	out := make([]entityRef, len(evicted))
	for i, e := range evicted {
		out[i] = e.entityRef
	}
	return out
}

func getEntity(s *state, rule int, uid string) (entity, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.rules[rule].entities[key(uid)]
	if !ok {
		return entity{}, false
	}
	return e.entity, true
}

func entityCount(s *state) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for i := range s.rules {
		n += len(s.rules[i].entities)
	}
	return n
}

// checkBytes asserts that the running size total matches the entities.
func checkBytes(t *testing.T, s *state) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	sum := 0
	for i := range s.rules {
		for k, e := range s.rules[i].entities {
			assert.Equal(t, entitySize(k, &e.entity), e.size)
			sum += e.size
		}
	}
	assert.Equal(t, sum, s.bytes)
	assert.LessOrEqual(t, s.bytes, s.maxStateBytes)
}

func TestStateApplyOutcomes(t *testing.T) {
	type step struct {
		o    observation
		want outcome
		// wantEntity is the expected entity after the step.
		wantEntity entity
	}
	identity := []kv{{Key: "k8s.pod.uid", Value: "u1"}}
	resource := []kv{{Key: "k8s.pod.name", Value: "pod-u1"}}
	v := func(version string) []kv { return []kv{{Key: "version", Value: version}} }
	renamed := obs(0, "u1", "2", 400)
	renamed.resource = []kv{{Key: "k8s.pod.name", Value: "renamed"}}

	steps := []step{
		{o: obs(0, "u1", "1", 100), want: outcomeInserted,
			wantEntity: entity{Identity: identity, Resource: resource, Facts: v("1"), FirstSeen: 100, Updated: 100, Activity: 100}},
		// Same facts later: Updated and Activity move, FirstSeen stays.
		{o: obs(0, "u1", "1", 200), want: outcomeUnchanged,
			wantEntity: entity{Identity: identity, Resource: resource, Facts: v("1"), FirstSeen: 100, Updated: 200, Activity: 200}},
		// Same facts at the same time.
		{o: obs(0, "u1", "1", 200), want: outcomeUnchanged,
			wantEntity: entity{Identity: identity, Resource: resource, Facts: v("1"), FirstSeen: 100, Updated: 200, Activity: 200}},
		// Different facts but older than Updated: ignored.
		{o: obs(0, "u1", "0", 150), want: outcomeIgnoredOlder,
			wantEntity: entity{Identity: identity, Resource: resource, Facts: v("1"), FirstSeen: 100, Updated: 200, Activity: 200}},
		// Same facts but older: also ignored-older (not unchanged).
		{o: obs(0, "u1", "1", 150), want: outcomeIgnoredOlder,
			wantEntity: entity{Identity: identity, Resource: resource, Facts: v("1"), FirstSeen: 100, Updated: 200, Activity: 200}},
		// New facts: replaced, new series starting at the record time.
		{o: obs(0, "u1", "2", 300), want: outcomeChanged,
			wantEntity: entity{Identity: identity, Resource: resource, Facts: v("2"), FirstSeen: 300, Updated: 300, Activity: 300}},
		// A changed resource value is also a change.
		{o: renamed, want: outcomeChanged,
			wantEntity: entity{Identity: identity, Resource: renamed.resource, Facts: v("2"), FirstSeen: 400, Updated: 400, Activity: 400}},
		// New facts at the same time as Updated are not older: changed.
		{o: obs(0, "u1", "3", 400), want: outcomeChanged,
			wantEntity: entity{Identity: identity, Resource: resource, Facts: v("3"), FirstSeen: 400, Updated: 400, Activity: 400}},
	}
	s := newState(testSpecs(10), defaultMaxStateBytes)
	for i, st := range steps {
		res := s.apply(st.o)
		assert.Equal(t, st.want, res.outcome, "step %d", i)
		assert.Empty(t, res.evicted, "step %d", i)
		got, ok := getEntity(s, 0, "u1")
		require.True(t, ok, "step %d", i)
		assert.Equal(t, st.wantEntity, got, "step %d", i)
		checkBytes(t, s)
	}
	_, ok := getEntity(s, 1, "u1")
	assert.False(t, ok, "rules have separate entities")
}

// TestStateOutOfOrderRestart models a container restart: kubelet keeps
// 0.log and 1.log, and the receiver can read them in any order. Both orders
// end with the facts of the newer file.
func TestStateOutOfOrderRestart(t *testing.T) {
	older := obs(0, "u1", "old", 100)
	newer := obs(0, "u1", "new", 200)
	for _, tc := range []struct {
		name  string
		order []observation
		want  []outcome
	}{
		{name: "older_first", order: []observation{older, newer}, want: []outcome{outcomeInserted, outcomeChanged}},
		{name: "newer_first", order: []observation{newer, older}, want: []outcome{outcomeInserted, outcomeIgnoredOlder}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newState(testSpecs(10), defaultMaxStateBytes)
			for i, o := range tc.order {
				assert.Equal(t, tc.want[i], s.apply(o).outcome)
			}
			got, _ := getEntity(s, 0, "u1")
			assert.Equal(t, []kv{{Key: "version", Value: "new"}}, got.Facts)
			assert.Equal(t, int64(200), got.FirstSeen)
			assert.Equal(t, int64(200), got.Updated)
			assert.Equal(t, int64(200), got.Activity)
		})
	}
}

func TestStateMaxEntitiesEviction(t *testing.T) {
	s := newState(testSpecs(3), defaultMaxStateBytes)
	require.Equal(t, outcomeInserted, s.apply(obs(0, "u1", "1", 100)).outcome)
	require.Equal(t, outcomeInserted, s.apply(obs(0, "u2", "1", 200)).outcome)
	require.Equal(t, outcomeInserted, s.apply(obs(0, "u3", "1", 300)).outcome)
	// The other rule's entities do not count against rule 0's limit and are
	// never evicted by it.
	require.Equal(t, outcomeInserted, s.apply(obs(1, "u0", "1", 1)).outcome)
	// u1 becomes the most active.
	require.Equal(t, outcomeUnchanged, s.apply(obs(0, "u1", "1", 400)).outcome)

	// At the limit, an insert evicts the least active entity: u2.
	res := s.apply(obs(0, "u4", "1", 500))
	assert.Equal(t, outcomeInserted, res.outcome)
	assert.Equal(t, []entityRef{ref(0, "u2")}, refs(res.evicted))
	// Then u3.
	res = s.apply(obs(0, "u5", "1", 50))
	assert.Equal(t, []entityRef{ref(0, "u3")}, refs(res.evicted))
	// A new entity is always admitted, even if it is older than the rest; it
	// is then the least active, so it goes first.
	res = s.apply(obs(0, "u6", "1", 600))
	assert.Equal(t, []entityRef{ref(0, "u5")}, refs(res.evicted))
	_, ok := getEntity(s, 1, "u0")
	assert.True(t, ok)
	assert.Equal(t, 4, entityCount(s))
	checkBytes(t, s)

	// Ties are broken by the smallest key.
	s = newState(testSpecs(2), defaultMaxStateBytes)
	s.apply(obs(0, "ub", "1", 100))
	s.apply(obs(0, "ua", "1", 100))
	res = s.apply(obs(0, "uc", "1", 100))
	assert.Equal(t, []entityRef{ref(0, "ua")}, refs(res.evicted))

	// A change does not evict for max_entities.
	res = s.apply(obs(0, "ub", "2", 200))
	assert.Equal(t, outcomeChanged, res.outcome)
	assert.Empty(t, res.evicted)
}

func TestEntitySize(t *testing.T) {
	o := obs(0, "u1", "1.2.3", 100)
	e := entity{Identity: o.identity, Resource: o.resource, Facts: o.facts}
	want := entityOverheadBytes + len(o.key) +
		pairOverheadBytes + len("k8s.pod.uid") + len("u1") +
		pairOverheadBytes + len("k8s.pod.name") + len("pod-u1") +
		pairOverheadBytes + len("version") + len("1.2.3")
	assert.Equal(t, want, entitySize(o.key, &e))
}

// TestEntitySizeEstimatesJSON checks that the size estimate is close to,
// and not below, the JSON encoding of a typical entity in a snapshot.
func TestEntitySizeEstimatesJSON(t *testing.T) {
	identity := []kv{{Key: "k8s.pod.uid", Value: "4b7f0c3e-9a1d-4e2b-8c3f-5d6e7f8a9b0c"}}
	e := entity{
		Identity: identity,
		Resource: []kv{{Key: "k8s.pod.name", Value: "vllm-main-7d9f"}, {Key: "k8s.namespace.name", Value: "inference"}, {Key: "k8s.container.name", Value: "vllm"}},
		Facts: []kv{
			{Key: "vllm_version", Value: "0.31.1"}, {Key: "model", Value: "meta-llama/Llama-3.1-70B-Instruct"},
			{Key: "vllm_engine", Value: "V1"}, {Key: "served_model_name", Value: "llama-70b"},
			{Key: "dtype", Value: "bfloat16"}, {Key: "max_seq_len", Value: "131072"},
			{Key: "tensor_parallel_size", Value: "4"}, {Key: "pipeline_parallel_size", Value: "2"},
			{Key: "data_parallel_size", Value: "1"}, {Key: "quantization", Value: "fp8"},
			{Key: "enforce_eager", Value: "False"}, {Key: "kv_cache_dtype", Value: "fp8_e4m3"},
			{Key: "enable_prefix_caching", Value: "True"}, {Key: "speculative_method", Value: "None"},
		},
		FirstSeen: 1760532001123456789,
		Updated:   1760532001123456789,
		Activity:  1760532001123456789,
	}
	k := encodeIdentity(identity)
	b, err := json.Marshal(map[string]entity{k: e})
	require.NoError(t, err)
	actual := len(b) - 2 // without the braces of the map
	estimate := entitySize(k, &e)
	assert.GreaterOrEqual(t, estimate, actual, "%s", b)
	assert.LessOrEqual(t, estimate, actual+64, "%s", b)

	// Values that encoding/json escapes count with their escaped length,
	// also in the state key.
	identity = []kv{{Key: "k8s.pod.uid", Value: `<u&1>"\`}}
	e.Identity = identity
	e.Facts = []kv{{Key: "model", Value: strings.Repeat(`<&>"\`+"\x01\n \xff", 50)}}
	k = encodeIdentity(identity)
	b, err = json.Marshal(map[string]entity{k: e})
	require.NoError(t, err)
	actual = len(b) - 2
	estimate = entitySize(k, &e)
	assert.GreaterOrEqual(t, estimate, actual)
	assert.LessOrEqual(t, estimate, actual+64)
}

// TestJSONStringLen compares jsonStringLen with encoding/json on every
// single byte, on characters that need escaping, and on random strings.
func TestJSONStringLen(t *testing.T) {
	inputs := []string{
		"", "plain", "meta-llama/Llama-3.1-70B-Instruct", `a"b\c`, "\b\f\n\r\t", "<>&", "\x00\x1f\x7f",
		"é日本🙂", "\u2028\u2029", "\ufffd", "\xff", "\xe6\x97", "abc\xe6\x97\xa5\xe6", "\xed\xa0\x80",
	}
	for b := range 256 {
		inputs = append(inputs, string([]byte{byte(b)}))
	}
	alphabet := []string{"a", "Z", "0", " ", `"`, `\`, "<", ">", "&", "\x00", "\n", "\x7f", "é", "日", "🙂", "\u2028", "\u2029", "\xff", "\xe6\x97", "\xc3"}
	rnd := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // G404: a fixed seed keeps the test reproducible.
	for range 2000 {
		var sb strings.Builder
		for range rnd.IntN(12) {
			sb.WriteString(alphabet[rnd.IntN(len(alphabet))])
		}
		inputs = append(inputs, sb.String())
	}
	for _, s := range inputs {
		b, err := json.Marshal(s)
		require.NoError(t, err)
		require.Equal(t, len(b)-2, jsonStringLen(s), "%q", s)
	}
}

func TestStateMaxStateBytesEviction(t *testing.T) {
	o := obs(0, "u1", "1", 0)
	size := entitySize(o.key, &entity{Identity: o.identity, Resource: o.resource, Facts: o.facts})
	// Room for three entities of this size.
	s := newState(testSpecs(100), 3*size+size/2)
	require.Equal(t, outcomeInserted, s.apply(obs(0, "u1", "1", 300)).outcome)
	require.Equal(t, outcomeInserted, s.apply(obs(1, "u2", "1", 100)).outcome)
	require.Equal(t, outcomeInserted, s.apply(obs(0, "u3", "1", 200)).outcome)
	checkBytes(t, s)

	// The fourth evicts the least active entity across all rules: u2 of rule 1.
	res := s.apply(obs(0, "u4", "1", 400))
	assert.Equal(t, outcomeInserted, res.outcome)
	assert.Equal(t, []entityRef{ref(1, "u2")}, refs(res.evicted))
	checkBytes(t, s)

	// A change that grows an entity evicts others, never the entity itself.
	// u4 grows by two entity sizes, so u3 (200) and u1 (300) go.
	grow := obs(0, "u4", strings.Repeat("v", 2*size), 500)
	res = s.apply(grow)
	assert.Equal(t, outcomeChanged, res.outcome)
	assert.Equal(t, []entityRef{ref(0, "u3"), ref(0, "u1")}, refs(res.evicted))
	got, ok := getEntity(s, 0, "u4")
	require.True(t, ok)
	assert.Equal(t, grow.facts, got.Facts)
	assert.Equal(t, 1, entityCount(s))
	checkBytes(t, s)
}

func TestStateEntityTooLarge(t *testing.T) {
	const maxBytes = 1000
	huge := strings.Repeat("x", maxBytes)
	s := newState(testSpecs(100), maxBytes)
	require.Equal(t, outcomeInserted, s.apply(obs(0, "u1", "1", 100)).outcome)

	// A new entity that alone exceeds the cap is not stored and evicts nothing.
	res := s.apply(obs(0, "u2", huge, 200))
	assert.Equal(t, outcomeTooLarge, res.outcome)
	assert.Empty(t, res.evicted)
	_, ok := getEntity(s, 0, "u2")
	assert.False(t, ok)
	_, ok = getEntity(s, 0, "u1")
	assert.True(t, ok)

	// A change that makes an entity too large removes its stale facts.
	res = s.apply(obs(0, "u1", huge, 300))
	assert.Equal(t, outcomeTooLarge, res.outcome)
	assert.Equal(t, []entityRef{ref(0, "u1")}, refs(res.evicted))
	assert.Equal(t, 0, entityCount(s))
	checkBytes(t, s)
	assert.Equal(t, 0, s.bytes)

	// An older too-large record is ignored like any older record.
	require.Equal(t, outcomeInserted, s.apply(obs(0, "u3", "1", 300)).outcome)
	assert.Equal(t, outcomeIgnoredOlder, s.apply(obs(0, "u3", huge, 100)).outcome)
}

func TestStateApplyBatch(t *testing.T) {
	s := newState(testSpecs(2), defaultMaxStateBytes)
	res := s.applyBatch([]observation{
		obs(0, "u1", "1", 100), // inserted
		obs(0, "u1", "2", 200), // changed in the same batch: emitted once, final value
		obs(0, "u1", "0", 150), // older: ignored
		obs(0, "u2", "1", 100), // inserted
		obs(1, "u1", "1", 100), // inserted, other rule
		obs(0, "u2", "1", 300), // unchanged
		obs(0, "u3", "1", 400), // inserted, evicts u1 (activity 200 < u2's 300)
	}, nil, true)
	assert.Equal(t, 4, res.counts[outcomeInserted])
	assert.Equal(t, 1, res.counts[outcomeChanged])
	assert.Equal(t, 1, res.counts[outcomeUnchanged])
	assert.Equal(t, 1, res.counts[outcomeIgnoredOlder])
	assert.Equal(t, []entityRef{ref(0, "u1")}, refs(res.evicted))
	// u1 of rule 0 was evicted later in the batch, so it is not emitted.
	require.Len(t, res.changed, 3)
	assert.Equal(t, 0, res.changed[0].rule)
	assert.Equal(t, "u2", res.changed[0].entity.Identity[0].Value)
	assert.Equal(t, 1, res.changed[1].rule)
	assert.Equal(t, "u1", res.changed[1].entity.Identity[0].Value)
	assert.Equal(t, int64(100), res.changed[1].entity.FirstSeen)
	assert.Equal(t, "u3", res.changed[2].entity.Identity[0].Value)

	// Without collect, nothing is copied.
	res = s.applyBatch([]observation{obs(0, "u9", "1", 500)}, nil, false)
	assert.Equal(t, 1, res.counts[outcomeInserted])
	assert.Empty(t, res.changed)

	// The final value of an entity changed twice in a batch is emitted.
	s = newState(testSpecs(2), defaultMaxStateBytes)
	res = s.applyBatch([]observation{obs(0, "u1", "1", 100), obs(0, "u1", "2", 200)}, nil, true)
	require.Len(t, res.changed, 1)
	assert.Equal(t, []kv{{Key: "version", Value: "2"}}, res.changed[0].entity.Facts)
	assert.Equal(t, int64(200), res.changed[0].entity.FirstSeen)
}

func TestStateSnapshotRestore(t *testing.T) {
	s := newState(testSpecs(10), defaultMaxStateBytes)
	s.apply(obs(0, "u1", "1", 100))
	s.apply(obs(0, "u1", "2", 200))
	s.apply(obs(0, "u2", "1", 150))
	s.apply(obs(1, "u3", "1", 300))
	s.apply(obs(1, "u3", "1", 350))

	snap := s.snapshot()
	b, err := json.Marshal(snap)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"rules":{"a":{"2:u1":{"identity":[{"key":"k8s.pod.uid","value":"u1"}],`+
		`"resource":[{"key":"k8s.pod.name","value":"pod-u1"}],"facts":[{"key":"version","value":"2"}],`+
		`"first_seen":200,"updated":200,"activity":200}`)

	var decoded stateSnapshot
	require.NoError(t, json.Unmarshal(b, &decoded))
	assert.Equal(t, snap, decoded)

	restored := newState(testSpecs(10), defaultMaxStateBytes)
	kept, dropped := restored.restore(decoded)
	assert.Equal(t, 3, kept)
	assert.Equal(t, 0, dropped)
	assert.Equal(t, snap, restored.snapshot())
	assert.Equal(t, s.bytes, restored.bytes)
	checkBytes(t, restored)

	// Restored entities keep their order protection: an older record is
	// still ignored.
	assert.Equal(t, outcomeIgnoredOlder, restored.apply(obs(0, "u1", "1", 150)).outcome)

	// A snapshot of an empty state has no rules.
	assert.Empty(t, newState(testSpecs(10), defaultMaxStateBytes).snapshot().Rules)
}

func TestStateRestoreDropsAndLimits(t *testing.T) {
	good := func(uid string, activity int64) entity {
		o := obs(0, uid, "1", activity)
		return entity{Identity: o.identity, Resource: o.resource, Facts: o.facts, FirstSeen: activity, Updated: activity, Activity: activity}
	}
	wrongKey := good("u9", 1)
	wrongKey.Identity = []kv{{Key: "k8s.pod.name", Value: "u9"}}
	emptyValue := good("u8", 1)
	emptyValue.Identity = []kv{{Key: "k8s.pod.uid", Value: ""}}
	snap := stateSnapshot{Rules: map[string]map[string]entity{
		"a": {
			key("u1"): good("u1", 100),
			key("u2"): good("u2", 300),
			key("u3"): good("u3", 200),
			// The state key does not match the identity.
			key("other"): good("u4", 400),
			// The identity keys do not match the rule.
			key("u9"): wrongKey,
			"0:":      emptyValue,
		},
		"b": {key("u5"): good("u5", 50)},
		// The rule is no longer configured.
		"removed": {key("u6"): good("u6", 500), key("u7"): good("u7", 500)},
	}}

	// max_entities 2 keeps the two most active entities of rule a.
	s := newState(testSpecs(2), defaultMaxStateBytes)
	kept, dropped := s.restore(snap)
	assert.Equal(t, 3, kept)
	assert.Equal(t, 6, dropped)
	for uid, want := range map[string]bool{"u1": false, "u2": true, "u3": true} {
		_, ok := getEntity(s, 0, uid)
		assert.Equal(t, want, ok, uid)
	}
	_, ok := getEntity(s, 1, "u5")
	assert.True(t, ok)
	checkBytes(t, s)

	// max_state_bytes keeps the most active entities across rules.
	size := entitySize(key("u1"), &entity{Identity: good("u1", 0).Identity, Resource: good("u1", 0).Resource, Facts: good("u1", 0).Facts})
	s = newState(testSpecs(10), 2*size)
	kept, dropped = s.restore(snap)
	assert.Equal(t, 2, kept)
	assert.Equal(t, 7, dropped)
	for uid, want := range map[string]bool{"u1": false, "u2": true, "u3": true} {
		_, ok := getEntity(s, 0, uid)
		assert.Equal(t, want, ok, uid)
	}
	checkBytes(t, s)

	// Restore replaces what was there.
	s = newState(testSpecs(10), defaultMaxStateBytes)
	s.apply(obs(0, "old", "1", 1))
	s.restore(stateSnapshot{})
	assert.Equal(t, 0, entityCount(s))
	assert.Equal(t, 0, s.bytes)
}

func TestStateManyEntities(t *testing.T) {
	// The defaults: 500 entities per rule, 1 MiB.
	s := newState(testSpecs(defaultMaxEntities), defaultMaxStateBytes)
	for i := 0; i < 2*defaultMaxEntities; i++ {
		s.apply(obs(i%2, fmt.Sprintf("uid-%04d", i), "1", int64(i)))
	}
	assert.Equal(t, 2*defaultMaxEntities, entityCount(s))
	for i := 0; i < 2; i++ {
		s.apply(obs(0, fmt.Sprintf("new-%d", i), "1", int64(10_000+i)))
	}
	// The two least active entities of rule 0 were evicted.
	_, ok := getEntity(s, 0, "uid-0000")
	assert.False(t, ok)
	_, ok = getEntity(s, 0, "uid-0002")
	assert.False(t, ok)
	_, ok = getEntity(s, 0, "uid-0004")
	assert.True(t, ok)
	checkBytes(t, s)
}

// touches returns an activityBatch with one touch per uid of rule, all at
// time t.
func touches(rule int, t int64, uids ...string) *activityBatch {
	act := &activityBatch{}
	for _, uid := range uids {
		from := len(act.keys)
		act.keys = append(act.keys, key(uid)...)
		act.touches = append(act.touches, touch{rule: rule, from: from, to: len(act.keys), time: t})
	}
	return act
}

func TestStateApplyBatchActivity(t *testing.T) {
	s := newState(testSpecs(2), defaultMaxStateBytes)
	s.apply(obs(0, "u1", "1", 100))
	s.apply(obs(0, "u2", "1", 200))

	// u1 is touched at 300 in the batch that inserts u3: the touches go
	// first, so u2 is the least active and is evicted.
	res := s.applyBatch([]observation{obs(0, "u3", "1", 400)}, touches(0, 300, "u1", "u3", "unknown"), false)
	assert.True(t, res.touched)
	assert.Equal(t, []eviction{{entityRef: ref(0, "u2"), reason: evictMaxEntities}}, res.evicted)
	got, ok := getEntity(s, 0, "u1")
	require.True(t, ok)
	assert.Equal(t, int64(300), got.Activity)
	assert.Equal(t, int64(100), got.Updated, "a touch changes only the activity")
	got, _ = getEntity(s, 0, "u3")
	assert.Equal(t, int64(400), got.Activity, "u3 did not exist when the touches were applied")

	// Older and equal touches do not move the activity, and a touch of
	// another rule does not apply to this rule.
	res = s.applyBatch(nil, touches(0, 300, "u1"), false)
	assert.False(t, res.touched)
	res = s.applyBatch(nil, touches(1, 500, "u1"), false)
	assert.False(t, res.touched)
	got, _ = getEntity(s, 0, "u1")
	assert.Equal(t, int64(300), got.Activity)

	// Observations report movement too: same facts later moves Updated
	// and Activity, an older record moves neither.
	res = s.applyBatch([]observation{obs(0, "u1", "1", 350)}, nil, false)
	assert.True(t, res.touched)
	assert.False(t, res.stateChanged())
	res = s.applyBatch([]observation{obs(0, "u1", "0", 200)}, nil, false)
	assert.False(t, res.touched)
	assert.False(t, res.stateChanged())
	// An older record with a later activity than stored moves the activity.
	s.apply(obs(0, "u1", "2", 1000))
	res = s.applyBatch(nil, touches(0, 2000, "u1"), false)
	assert.True(t, res.touched)
	res = s.applyBatch([]observation{obs(0, "u1", "3", 3000)}, nil, false)
	assert.True(t, res.stateChanged())
}

func TestStateEvictionReasons(t *testing.T) {
	o := obs(0, "u1", "1", 0)
	size := entitySize(o.key, &entity{Identity: o.identity, Resource: o.resource, Facts: o.facts})
	s := newState(testSpecs(2), 3*size+size/2)
	s.apply(obs(0, "u1", "1", 100))
	s.apply(obs(0, "u2", "1", 200))
	s.apply(obs(1, "u3", "1", 300))

	res := s.applyBatch([]observation{
		obs(0, "u4", "1", 400),                         // max_entities evicts u1
		obs(1, "u5", "1", 500),                         // max_state_bytes evicts u2
		obs(1, "u6", strings.Repeat("x", 4*size), 1),   // too large, not stored
		obs(1, "u5", strings.Repeat("x", 4*size), 600), // too large, removed
	}, nil, false)
	assert.Equal(t, []eviction{
		{entityRef: ref(0, "u1"), reason: evictMaxEntities},
		{entityRef: ref(0, "u2"), reason: evictMaxStateBytes},
		{entityRef: ref(1, "u5"), reason: evictTooLarge},
	}, res.evicted)
	assert.Equal(t, []entityRef{ref(1, "u6")}, res.rejected)
	assert.Equal(t, 2, res.counts[outcomeTooLarge])
	assert.True(t, res.stateChanged())
	checkBytes(t, s)
}

func TestStateTracked(t *testing.T) {
	s := newState(testSpecs(2), defaultMaxStateBytes)
	tracked := func() []int64 {
		return []int64{s.tracked[0].Load(), s.tracked[1].Load()}
	}
	assert.False(t, s.tracking())
	s.apply(obs(1, "u1", "1", 100))
	assert.True(t, s.tracking())
	assert.False(t, s.ruleTracked(0))
	assert.True(t, s.ruleTracked(1))
	s.apply(obs(1, "u2", "1", 200))
	s.apply(obs(1, "u3", "1", 300))
	assert.Equal(t, []int64{0, 2}, tracked(), "max_entities evicted one")
	s.apply(obs(1, "u3", strings.Repeat("x", defaultMaxStateBytes), 400))
	assert.Equal(t, []int64{0, 1}, tracked(), "too large removed one")

	snap := s.snapshot()
	s.apply(obs(0, "u9", "1", 1))
	s.restore(snap)
	assert.Equal(t, []int64{0, 1}, tracked())
	s.restore(stateSnapshot{})
	assert.Equal(t, []int64{0, 0}, tracked())
	assert.False(t, s.tracking())
}

func TestStateEntriesAndSizes(t *testing.T) {
	s := newState(testSpecs(10), defaultMaxStateBytes)
	assert.Nil(t, s.entries([]int{0, 1}))
	s.apply(obs(1, "u2", "1", 100))
	s.apply(obs(0, "u3", "1", 100))
	s.apply(obs(1, "u1", "1", 100))
	s.apply(obs(0, "u1", "2", 100))

	got := s.entries([]int{1, 0})
	var names []string
	for _, e := range got {
		names = append(names, fmt.Sprintf("%d/%s/%s", e.rule, e.entity.Identity[0].Value, e.entity.Facts[0].Value))
		assert.Equal(t, encodeIdentity(e.entity.Identity), e.key)
	}
	assert.Equal(t, []string{"0/u1/2", "0/u3/1", "1/u1/1", "1/u2/1"}, names)
	assert.Len(t, s.entries([]int{1}), 2)

	counts := make([]int, 2)
	assert.Equal(t, s.bytes, s.sizes(counts))
	assert.Equal(t, []int{2, 2}, counts)
}
