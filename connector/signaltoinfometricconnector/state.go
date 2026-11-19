// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"cmp"
	"slices"
	"sync"
	"sync/atomic"
	"unicode/utf8"
)

// Size estimate of the serialized state, used for max_state_bytes. The
// estimate of one entity is the sum of the JSON-encoded lengths of its state
// key and of every key and value it holds, plus these constants. They cover
// the rest of the JSON encoding of an entity in a snapshot, with the comma
// that separates it from the next, so the estimate is at least the encoded
// size.
const (
	// entityOverheadBytes covers the field names, brackets, and separators
	// of one entity, and its three timestamps at 19 digits each:
	// "<key>":{"identity":[],"resource":[],"facts":[],"first_seen":N,"updated":N,"activity":N},
	entityOverheadBytes = 160
	// pairOverheadBytes covers the framing of one key/value pair:
	// {"key":"","value":""},
	pairOverheadBytes = 24
)

// kv is one key/value pair. Lists of pairs keep the configured order.
type kv struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// entity is the latest known facts for one identity of one rule. The state
// never modifies a slice of an entity after storing it; a change stores new
// slices. Copies of an entity can therefore share its slices, and a snapshot
// can be encoded without holding the state lock.
type entity struct {
	// Identity holds the identity keys and values, in the rule's order.
	Identity []kv `json:"identity"`
	// Resource holds the included resource attributes that were present,
	// in the rule's order.
	Resource []kv `json:"resource,omitempty"`
	// Facts holds the extracted attributes, in the rule's order. Optional
	// attributes without a value are absent.
	Facts []kv `json:"facts"`
	// FirstSeen is the record time (Unix nanoseconds) at which the current
	// facts and resource values were first seen. It is the start time of
	// the emitted series and is reset when they change.
	FirstSeen int64 `json:"first_seen"`
	// Updated is the time of the latest record whose facts were applied.
	// Records older than Updated do not replace the facts.
	Updated int64 `json:"updated"`
	// Activity is the time of the latest record seen for the entity. The
	// least active entities are evicted first.
	Activity int64 `json:"activity"`
}

// observation is what a matching record says about an entity.
type observation struct {
	rule     int
	key      string
	identity []kv
	resource []kv
	facts    []kv
	// time is the record time in Unix nanoseconds.
	time int64
}

// outcome is the result of applying one observation.
type outcome uint8

const (
	// outcomeInserted: a new entity was stored.
	outcomeInserted outcome = iota + 1
	// outcomeChanged: the facts or resource values changed and replaced
	// the previous ones; FirstSeen was reset.
	outcomeChanged
	// outcomeUnchanged: same facts and resource values; Updated and
	// Activity moved forward.
	outcomeUnchanged
	// outcomeIgnoredOlder: the record is older than the stored facts, so
	// only Activity was updated.
	outcomeIgnoredOlder
	// outcomeTooLarge: the entity alone exceeds max_state_bytes and was
	// not stored. If it existed, its stale facts were removed.
	outcomeTooLarge

	numOutcomes = int(outcomeTooLarge) + 1
)

// entityRef identifies an entity in the state.
type entityRef struct {
	rule int
	key  string
}

// evictReason is why an entity left the state.
type evictReason uint8

const (
	// evictMaxEntities: a new entity of the same rule needed room.
	evictMaxEntities evictReason = iota
	// evictMaxStateBytes: the state was over max_state_bytes.
	evictMaxStateBytes
	// evictTooLarge: a change made the entity alone exceed max_state_bytes.
	evictTooLarge

	numEvictReasons
)

// eviction is an entity removed from the state.
type eviction struct {
	entityRef
	reason evictReason
}

// applyResult is the result of applying one observation.
type applyResult struct {
	outcome outcome
	// evicted lists the entities removed to make room, and the entity
	// itself when a change made it too large.
	evicted []eviction
	// touched is set when the Activity or Updated time of an existing
	// entity moved forward.
	touched bool
}

// emitEntry is a copy of an entity to emit.
type emitEntry struct {
	rule   int
	key    string
	entity entity
}

// batchResult summarizes applyBatch.
type batchResult struct {
	// counts is indexed by outcome.
	counts  [numOutcomes]int
	evicted []eviction
	// rejected lists the new entities that were not stored because each
	// alone exceeds max_state_bytes.
	rejected []entityRef
	// changed holds copies of the entities inserted or changed by the
	// batch that still exist after it, in order of their first change.
	// It is filled only when requested.
	changed []emitEntry
	// touched is set when the batch moved the Activity or Updated time of
	// an existing entity forward.
	touched bool
}

// stateChanged reports whether the batch changed the stored facts or the
// set of entities: an insert, a change, an eviction, or an entity that was
// too large.
func (r *batchResult) stateChanged() bool {
	return r.counts[outcomeInserted] > 0 || r.counts[outcomeChanged] > 0 ||
		r.counts[outcomeTooLarge] > 0 || len(r.evicted) > 0
}

// touch says that the entity of a rule whose state key is keys[from:to]
// had a record at time (Unix nanoseconds).
type touch struct {
	rule     int
	from, to int
	time     int64
}

// activityBatch holds the touches of one ConsumeLogs call. The keys of all
// touches share one buffer, so that collecting them does not allocate once
// the buffers have grown.
type activityBatch struct {
	keys    []byte
	touches []touch
	// now is the current time in Unix nanoseconds, read once per batch
	// when the first touch needs it. 0 means not read yet.
	now int64
}

// maxPooledKeyBytes bounds the key buffer that an activityBatch keeps in the
// pool, so that one batch with many long keys does not pin memory.
const maxPooledKeyBytes = 64 << 10

var activityPool = sync.Pool{New: func() any { return new(activityBatch) }}

func getActivityBatch() *activityBatch {
	act, _ := activityPool.Get().(*activityBatch)
	return act
}

func putActivityBatch(act *activityBatch) {
	if act == nil || cap(act.keys) > maxPooledKeyBytes {
		return
	}
	act.keys = act.keys[:0]
	act.touches = act.touches[:0]
	act.now = 0
	activityPool.Put(act)
}

// ruleSpec is what the state needs to know about a rule.
type ruleSpec struct {
	name        string
	identity    []string
	maxEntities int
}

type storedEntity struct {
	entity
	// size is the estimate of the entity's serialized size.
	size int
}

type ruleState struct {
	ruleSpec
	entities map[string]*storedEntity
}

// state holds the entities of every rule. It does not use pdata types. One
// mutex protects it; callers prepare observations before taking the lock.
type state struct {
	mu            sync.Mutex
	rules         []ruleState
	ruleIndex     map[string]int
	maxStateBytes int
	// bytes is the sum of the size estimates of all entities.
	bytes int
	// tracked mirrors the number of entities of each rule. It is written
	// under the lock and read without it, so that ConsumeLogs skips the
	// activity lookup of a rule that has no entities without locking.
	tracked []atomic.Int64
}

func newState(specs []ruleSpec, maxStateBytes int) *state {
	s := &state{
		rules:         make([]ruleState, len(specs)),
		ruleIndex:     make(map[string]int, len(specs)),
		maxStateBytes: maxStateBytes,
		tracked:       make([]atomic.Int64, len(specs)),
	}
	for i, spec := range specs {
		s.rules[i] = ruleState{ruleSpec: spec, entities: make(map[string]*storedEntity)}
		s.ruleIndex[spec.name] = i
	}
	return s
}

// apply applies one observation.
func (s *state) apply(o observation) applyResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applyLocked(&o)
}

// tracking reports whether any rule has entities. It does not lock.
func (s *state) tracking() bool {
	for i := range s.tracked {
		if s.tracked[i].Load() > 0 {
			return true
		}
	}
	return false
}

// ruleTracked reports whether the rule has entities. It does not lock.
func (s *state) ruleTracked(rule int) bool {
	return s.tracked[rule].Load() > 0
}

// applyBatch applies the touches of act, which can be nil, then the
// observations in order, under one lock. The touches go first, so that an
// eviction caused by the observations sees the latest activity. When
// collect is set, it also returns copies of the entities that the batch
// inserted or changed, each once with its final value.
func (s *state) applyBatch(obs []observation, act *activityBatch, collect bool) batchResult {
	var res batchResult
	var dirty []entityRef
	var seen map[entityRef]struct{}

	s.mu.Lock()
	defer s.mu.Unlock()
	if act != nil {
		res.touched = s.touchLocked(act)
	}
	for i := range obs {
		r := s.applyLocked(&obs[i])
		res.counts[r.outcome]++
		res.evicted = append(res.evicted, r.evicted...)
		res.touched = res.touched || r.touched
		if r.outcome == outcomeTooLarge && len(r.evicted) == 0 {
			res.rejected = append(res.rejected, entityRef{rule: obs[i].rule, key: obs[i].key})
		}
		if !collect || (r.outcome != outcomeInserted && r.outcome != outcomeChanged) {
			continue
		}
		ref := entityRef{rule: obs[i].rule, key: obs[i].key}
		if seen == nil {
			seen = make(map[entityRef]struct{})
		}
		if _, ok := seen[ref]; !ok {
			seen[ref] = struct{}{}
			dirty = append(dirty, ref)
		}
	}
	for _, ref := range dirty {
		if e, ok := s.rules[ref.rule].entities[ref.key]; ok {
			res.changed = append(res.changed, emitEntry{rule: ref.rule, key: ref.key, entity: e.entity})
		}
	}
	return res
}

// touchLocked moves the Activity of each touched entity that exists
// forward to the touch time. It reports whether any Activity moved. The
// map index with a converted byte slice does not allocate.
func (s *state) touchLocked(act *activityBatch) bool {
	moved := false
	for i := range act.touches {
		t := &act.touches[i]
		e, ok := s.rules[t.rule].entities[string(act.keys[t.from:t.to])]
		if ok && t.time > e.Activity {
			e.Activity = t.time
			moved = true
		}
	}
	return moved
}

// entries returns copies of the entities of the given rules, sorted by
// rule index and then by key. The copies share their slices with the
// state, which never modifies them, so the caller can use them without the
// lock.
func (s *state) entries(rules []int) []emitEntry {
	out := s.copyEntries(rules)
	slices.SortFunc(out, func(a, b emitEntry) int {
		return cmp.Or(cmp.Compare(a.rule, b.rule), cmp.Compare(a.key, b.key))
	})
	return out
}

func (s *state) copyEntries(rules []int) []emitEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, ri := range rules {
		n += len(s.rules[ri].entities)
	}
	if n == 0 {
		return nil
	}
	out := make([]emitEntry, 0, n)
	for _, ri := range rules {
		for key, e := range s.rules[ri].entities {
			out = append(out, emitEntry{rule: ri, key: key, entity: e.entity})
		}
	}
	return out
}

// sizes stores the number of entities of each rule in counts, which has one
// element per rule, and returns the size estimate of the state.
func (s *state) sizes(counts []int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.rules {
		counts[i] = len(s.rules[i].entities)
	}
	return s.bytes
}

func (s *state) applyLocked(o *observation) applyResult {
	rs := &s.rules[o.rule]
	cur, exists := rs.entities[o.key]
	if !exists {
		return s.insertLocked(o.rule, o.key, entity{
			Identity:  o.identity,
			Resource:  o.resource,
			Facts:     o.facts,
			FirstSeen: o.time,
			Updated:   o.time,
			Activity:  o.time,
		})
	}

	touched := o.time > cur.Activity
	cur.Activity = max(cur.Activity, o.time)
	if o.time < cur.Updated {
		return applyResult{outcome: outcomeIgnoredOlder, touched: touched}
	}
	if slices.Equal(cur.Facts, o.facts) && slices.Equal(cur.Resource, o.resource) {
		touched = touched || o.time > cur.Updated
		cur.Updated = o.time
		return applyResult{outcome: outcomeUnchanged, touched: touched}
	}

	next := entity{
		Identity:  o.identity,
		Resource:  o.resource,
		Facts:     o.facts,
		FirstSeen: o.time,
		Updated:   o.time,
		Activity:  cur.Activity,
	}
	size := entitySize(o.key, &next)
	if size > s.maxStateBytes {
		// Keeping the previous facts would emit values known to be stale.
		ref := entityRef{rule: o.rule, key: o.key}
		s.removeLocked(ref)
		return applyResult{outcome: outcomeTooLarge, evicted: []eviction{{entityRef: ref, reason: evictTooLarge}}}
	}
	s.bytes += size - cur.size
	cur.entity, cur.size = next, size
	return applyResult{outcome: outcomeChanged, evicted: s.evictForBytesLocked(o.rule, o.key)}
}

// insertLocked stores a new entity, evicting the rule's least active entity
// at max_entities, then the least active entities of any rule while the
// state is over max_state_bytes.
func (s *state) insertLocked(rule int, key string, e entity) applyResult {
	size := entitySize(key, &e)
	if size > s.maxStateBytes {
		return applyResult{outcome: outcomeTooLarge}
	}
	rs := &s.rules[rule]
	var evicted []eviction
	for len(rs.entities) >= rs.maxEntities {
		victim, ok := s.leastActiveLocked(rule, rule, "")
		if !ok {
			break
		}
		s.removeLocked(victim)
		evicted = append(evicted, eviction{entityRef: victim, reason: evictMaxEntities})
	}
	rs.entities[key] = &storedEntity{entity: e, size: size}
	s.tracked[rule].Store(int64(len(rs.entities)))
	s.bytes += size
	evicted = append(evicted, s.evictForBytesLocked(rule, key)...)
	return applyResult{outcome: outcomeInserted, evicted: evicted}
}

// evictForBytesLocked evicts the least active entities of any rule, never
// the entity (keepRule, keepKey), until the state fits max_state_bytes.
func (s *state) evictForBytesLocked(keepRule int, keepKey string) []eviction {
	var evicted []eviction
	for s.bytes > s.maxStateBytes {
		victim, ok := s.leastActiveLocked(-1, keepRule, keepKey)
		if !ok {
			break
		}
		s.removeLocked(victim)
		evicted = append(evicted, eviction{entityRef: victim, reason: evictMaxStateBytes})
	}
	return evicted
}

// leastActiveLocked returns the entity with the smallest Activity, ties
// broken by the smallest rule name and then the smallest key. It scans one
// rule, or all rules when rule is -1, and skips the entity (skipRule,
// skipKey) when skipKey is not empty. The scan is O(n), which is fine for
// the configured limits because eviction is rare.
func (s *state) leastActiveLocked(rule, skipRule int, skipKey string) (entityRef, bool) {
	var best entityRef
	var bestActivity int64
	found := false
	for ri := range s.rules {
		if rule >= 0 && ri != rule {
			continue
		}
		for key, e := range s.rules[ri].entities {
			if skipKey != "" && ri == skipRule && key == skipKey {
				continue
			}
			if !found || e.Activity < bestActivity ||
				(e.Activity == bestActivity && s.lessRef(entityRef{rule: ri, key: key}, best)) {
				best, bestActivity, found = entityRef{rule: ri, key: key}, e.Activity, true
			}
		}
	}
	return best, found
}

func (s *state) lessRef(a, b entityRef) bool {
	return cmp.Or(
		cmp.Compare(s.rules[a.rule].name, s.rules[b.rule].name),
		cmp.Compare(a.key, b.key),
	) < 0
}

func (s *state) removeLocked(ref entityRef) {
	rs := &s.rules[ref.rule]
	if e, ok := rs.entities[ref.key]; ok {
		s.bytes -= e.size
		delete(rs.entities, ref.key)
		s.tracked[ref.rule].Store(int64(len(rs.entities)))
	}
}

// entitySize estimates the serialized size of an entity; see
// entityOverheadBytes.
func entitySize(key string, e *entity) int {
	return entityOverheadBytes + jsonStringLen(key) + pairsSize(e.Identity) + pairsSize(e.Resource) + pairsSize(e.Facts)
}

func pairsSize(pairs []kv) int {
	n := 0
	for _, p := range pairs {
		n += pairOverheadBytes + jsonStringLen(p.Key) + jsonStringLen(p.Value)
	}
	return n
}

// jsonStringLen returns the length of s as encoding/json writes it in a
// string, without the quotes. It is len(s) unless s holds a character that
// encoding/json escapes: '"', '\', and \b, \f, \n, \r, \t take 2 bytes;
// other control characters and '<', '>', '&' take 6 (\u00XX); an invalid
// UTF-8 byte takes 6 (\ufffd); U+2028 and U+2029 take 6.
func jsonStringLen(s string) int {
	n := 0
	for i := 0; i < len(s); {
		b := s[i]
		if b < utf8.RuneSelf {
			switch {
			case b == '"' || b == '\\' || b == '\b' || b == '\f' || b == '\n' || b == '\r' || b == '\t':
				n += 2
			case b < 0x20 || b == '<' || b == '>' || b == '&':
				n += 6
			default:
				n++
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if (r == utf8.RuneError && size == 1) || r == '\u2028' || r == '\u2029' {
			n += 6
		} else {
			n += size
		}
		i += size
	}
	return n
}

// stateSnapshot is the serializable form of the state: rule name, then
// identity key, then entity. Rules without entities are omitted.
type stateSnapshot struct {
	Rules map[string]map[string]entity `json:"rules"`
}

// snapshot returns a copy of the state. The entities share their slices with
// the state, which never modifies them, so the snapshot can be encoded
// without the lock.
func (s *state) snapshot() stateSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := stateSnapshot{Rules: make(map[string]map[string]entity, len(s.rules))}
	for i := range s.rules {
		rs := &s.rules[i]
		if len(rs.entities) == 0 {
			continue
		}
		entities := make(map[string]entity, len(rs.entities))
		for key, e := range rs.entities {
			entities[key] = e.entity
		}
		snap.Rules[rs.name] = entities
	}
	return snap
}

// restore replaces the state with the entities of a snapshot and returns
// how many it kept and dropped. It drops entities of rules that are no
// longer configured and entities whose identity does not match the rule's
// identity keys or the state key. It then enforces max_entities and
// max_state_bytes, keeping the most active entities. The state takes
// ownership of the snapshot's slices.
func (s *state) restore(snap stateSnapshot) (int, int) {
	type item struct {
		rule int
		key  string
		e    entity
	}
	var items []item
	dropped := 0
	for name, entities := range snap.Rules {
		ri, ok := s.ruleIndex[name]
		if !ok {
			dropped += len(entities)
			continue
		}
		for key, e := range entities {
			if !s.rules[ri].valid(key, &e) {
				dropped++
				continue
			}
			items = append(items, item{rule: ri, key: key, e: e})
		}
	}
	// Insert the least active first, so that the limits evict them.
	slices.SortFunc(items, func(a, b item) int {
		return cmp.Or(
			cmp.Compare(a.e.Activity, b.e.Activity),
			cmp.Compare(s.rules[a.rule].name, s.rules[b.rule].name),
			cmp.Compare(a.key, b.key),
		)
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.rules {
		s.rules[i].entities = make(map[string]*storedEntity)
		s.tracked[i].Store(0)
	}
	s.bytes = 0
	for _, it := range items {
		r := s.insertLocked(it.rule, it.key, it.e)
		dropped += len(r.evicted)
		if r.outcome == outcomeTooLarge {
			dropped++
		}
	}
	kept := 0
	for i := range s.rules {
		kept += len(s.rules[i].entities)
	}
	return kept, dropped
}

// valid reports whether a restored entity belongs to the rule: its identity
// keys are the rule's identity keys in order, its values are not empty, and
// key is their encoding.
func (rs *ruleState) valid(key string, e *entity) bool {
	if len(e.Identity) != len(rs.identity) {
		return false
	}
	for i, p := range e.Identity {
		if p.Key != rs.identity[i] || p.Value == "" {
			return false
		}
	}
	return key == encodeIdentity(e.Identity)
}
