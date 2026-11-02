// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"bytes"
	"cmp"
	"encoding/json"
	"slices"
	"sync"
	"unicode/utf8"
)

// The size estimate of an entry is at least its encoded size in the stored state.
const (
	entryOverheadBytes = 96 // "<key>":{"identity":[],"record":,"source_file":"","time":N},
	pairOverheadBytes  = 24 // {"key":"","value":""},
)

type kv struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// The state never modifies the slices of a stored entry, so that copies can share them and a
// snapshot can be encoded without the lock.
type entry struct {
	Identity   []kv            `json:"identity"`
	Record     json.RawMessage `json:"record"`
	SourceFile string          `json:"source_file,omitempty"`
	Time       int64           `json:"time"`
}

type observation struct {
	rule       int
	key        string
	identity   []kv
	record     []byte
	sourceFile string
	time       int64
	tooLarge   bool
}

type outcome uint8

const (
	outcomeInserted outcome = iota
	outcomeReplaced
	outcomeUnchanged
	outcomeIgnoredOlder
	// outcomeTooLarge: over max_record_bytes, or alone over max_state_bytes.
	outcomeTooLarge
	// outcomeOverLimit: older than every entry of a full rule.
	outcomeOverLimit

	numOutcomes
)

type entryRef struct {
	rule int
	key  string
}

type removeReason uint8

const (
	removeSourceGone removeReason = iota
	removeMaxEntries
	removeMaxStateBytes
	removeTooLarge

	numRemoveReasons
)

var removeReasonNames = [numRemoveReasons]string{"source_gone", "max_entries", "max_state_bytes", "too_large"}

type removal struct {
	entryRef
	reason removeReason
}

type batchResult struct {
	counts   [numOutcomes]int // indexed by outcome
	removed  []removal
	rejected []removal
}

func (r *batchResult) changed() bool {
	return r.counts[outcomeInserted] > 0 || r.counts[outcomeReplaced] > 0 || len(r.removed) > 0
}

type applyResult struct {
	outcome  outcome
	removed  []removal
	rejected *removal
}

type ruleSpec struct {
	name       string
	identity   []string
	maxEntries int
}

type storedEntry struct {
	entry
	size int // size estimate
}

type ruleState struct {
	ruleSpec
	entries map[string]*storedEntry
}

type state struct {
	mu            sync.Mutex
	rules         []ruleState
	ruleIndex     map[string]int
	maxStateBytes int
	bytes         int // sum of the size estimates
	lastReplay    int64
}

func newState(specs []ruleSpec, maxStateBytes int) *state {
	s := &state{
		rules:         make([]ruleState, len(specs)),
		ruleIndex:     make(map[string]int, len(specs)),
		maxStateBytes: maxStateBytes,
	}
	for i, spec := range specs {
		s.rules[i] = ruleState{ruleSpec: spec, entries: make(map[string]*storedEntry)}
		s.ruleIndex[spec.name] = i
	}
	return s
}

func (s *state) applyBatch(obs []observation) batchResult {
	var res batchResult
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range obs {
		r := s.applyLocked(&obs[i])
		res.counts[r.outcome]++
		res.removed = append(res.removed, r.removed...)
		if r.rejected != nil {
			res.rejected = append(res.rejected, *r.rejected)
		}
	}
	return res
}

func (s *state) applyLocked(o *observation) applyResult {
	ref := entryRef{rule: o.rule, key: o.key}
	rs := &s.rules[o.rule]
	cur, exists := rs.entries[o.key]
	if o.tooLarge {
		// A kept record that is not newer is stale, so it goes too.
		if exists && o.time >= cur.Time {
			s.removeLocked(ref)
			return applyResult{outcome: outcomeTooLarge, removed: []removal{{entryRef: ref, reason: removeTooLarge}}}
		}
		return applyResult{outcome: outcomeTooLarge}
	}
	next := entry{Identity: o.identity, Record: o.record, SourceFile: o.sourceFile, Time: o.time}
	if !exists {
		return s.insertLocked(o.rule, o.key, next)
	}
	if o.time < cur.Time {
		return applyResult{outcome: outcomeIgnoredOlder}
	}
	if o.time == cur.Time && o.sourceFile == cur.SourceFile && bytes.Equal(o.record, cur.Record) {
		return applyResult{outcome: outcomeUnchanged}
	}
	size := entrySize(o.key, &next)
	if size > s.maxStateBytes {
		// Keeping the previous record would send a record known to be stale.
		s.removeLocked(ref)
		return applyResult{outcome: outcomeTooLarge, removed: []removal{{entryRef: ref, reason: removeTooLarge}}}
	}
	s.bytes += size - cur.size
	cur.entry, cur.size = next, size
	return applyResult{outcome: outcomeReplaced, removed: s.evictForBytesLocked(o.rule, o.key)}
}

func (s *state) insertLocked(rule int, key string, e entry) applyResult {
	self := entryRef{rule: rule, key: key}
	size := entrySize(key, &e)
	if size > s.maxStateBytes {
		return applyResult{outcome: outcomeTooLarge, rejected: &removal{entryRef: self, reason: removeTooLarge}}
	}
	rs := &s.rules[rule]
	var removed []removal
	for len(rs.entries) >= rs.maxEntries {
		victim, ok := s.oldestLocked(rule, rule, "")
		if !ok {
			break
		}
		if e.Time < rs.entries[victim.key].Time {
			return applyResult{outcome: outcomeOverLimit, removed: removed, rejected: &removal{entryRef: self, reason: removeMaxEntries}}
		}
		s.removeLocked(victim)
		removed = append(removed, removal{entryRef: victim, reason: removeMaxEntries})
	}
	rs.entries[key] = &storedEntry{entry: e, size: size}
	s.bytes += size
	return applyResult{outcome: outcomeInserted, removed: append(removed, s.evictForBytesLocked(rule, key)...)}
}

func (s *state) evictForBytesLocked(keepRule int, keepKey string) []removal {
	var removed []removal
	for s.bytes > s.maxStateBytes {
		victim, ok := s.oldestLocked(-1, keepRule, keepKey)
		if !ok {
			break
		}
		s.removeLocked(victim)
		removed = append(removed, removal{entryRef: victim, reason: removeMaxStateBytes})
	}
	return removed
}

// oldestLocked scans one rule, or all rules for rule -1. Eviction is rare, so the scan is O(n).
func (s *state) oldestLocked(rule, skipRule int, skipKey string) (entryRef, bool) {
	var best entryRef
	var bestTime int64
	found := false
	for ri := range s.rules {
		if rule >= 0 && ri != rule {
			continue
		}
		for key, e := range s.rules[ri].entries {
			if skipKey != "" && ri == skipRule && key == skipKey {
				continue
			}
			ref := entryRef{rule: ri, key: key}
			if !found || e.Time < bestTime || (e.Time == bestTime && s.lessRef(ref, best)) {
				best, bestTime, found = ref, e.Time, true
			}
		}
	}
	return best, found
}

func (s *state) lessRef(a, b entryRef) bool {
	return cmp.Or(
		cmp.Compare(s.rules[a.rule].name, s.rules[b.rule].name),
		cmp.Compare(a.key, b.key),
	) < 0
}

func (s *state) removeLocked(ref entryRef) {
	rs := &s.rules[ref.rule]
	if e, ok := rs.entries[ref.key]; ok {
		s.bytes -= e.size
		delete(rs.entries, ref.key)
	}
}

type keptEntry struct {
	entryRef
	entry entry
}

func (s *state) entries() []keptEntry {
	s.mu.Lock()
	n := 0
	for ri := range s.rules {
		n += len(s.rules[ri].entries)
	}
	out := make([]keptEntry, 0, n)
	for ri := range s.rules {
		for key, e := range s.rules[ri].entries {
			out = append(out, keptEntry{entryRef: entryRef{rule: ri, key: key}, entry: e.entry})
		}
	}
	s.mu.Unlock()
	slices.SortFunc(out, func(a, b keptEntry) int {
		return cmp.Or(cmp.Compare(a.rule, b.rule), cmp.Compare(a.key, b.key))
	})
	return out
}

// removeGone keeps an entry that a newer record replaced during the check.
func (s *state) removeGone(gone []keptEntry) []removal {
	if len(gone) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var removed []removal
	for i := range gone {
		g := &gone[i]
		cur, ok := s.rules[g.rule].entries[g.key]
		if !ok || cur.Time != g.entry.Time || cur.SourceFile != g.entry.SourceFile {
			continue
		}
		s.removeLocked(g.entryRef)
		removed = append(removed, removal{entryRef: g.entryRef, reason: removeSourceGone})
	}
	return removed
}

func entrySize(key string, e *entry) int {
	n := entryOverheadBytes + jsonStringLen(key) + len(e.Record) + jsonStringLen(e.SourceFile)
	for _, p := range e.Identity {
		n += pairOverheadBytes + jsonStringLen(p.Key) + jsonStringLen(p.Value)
	}
	return n
}

// jsonStringLen returns the length of s as encoding/json writes it in a string, without the quotes.
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

// stateSnapshot is the stored form: rule name, then identity key, then entry.
type stateSnapshot struct {
	Rules      map[string]map[string]entry `json:"rules"`
	LastReplay int64                       `json:"last_replay,omitempty"`
}

func (s *state) setLastReplay(t int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastReplay = t
}

func (s *state) lastReplayTime() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastReplay
}

func (s *state) snapshot() stateSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := stateSnapshot{Rules: make(map[string]map[string]entry, len(s.rules)), LastReplay: s.lastReplay}
	for i := range s.rules {
		rs := &s.rules[i]
		if len(rs.entries) == 0 {
			continue
		}
		entries := make(map[string]entry, len(rs.entries))
		for key, e := range rs.entries {
			entries[key] = e.entry
		}
		snap.Rules[rs.name] = entries
	}
	return snap
}

type restoredEntry struct {
	entryRef
	entry entry
}

func (s *state) restore(items []restoredEntry, lastReplay int64) (int, int) {
	// Insert the oldest first, so that the limits evict them.
	slices.SortFunc(items, func(a, b restoredEntry) int {
		return cmp.Or(
			cmp.Compare(a.entry.Time, b.entry.Time),
			cmp.Compare(s.rules[a.rule].name, s.rules[b.rule].name),
			cmp.Compare(a.key, b.key),
		)
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.rules {
		s.rules[i].entries = make(map[string]*storedEntry)
	}
	s.bytes = 0
	s.lastReplay = lastReplay
	dropped := 0
	for _, it := range items {
		r := s.insertLocked(it.rule, it.key, it.entry)
		dropped += len(r.removed)
		if r.rejected != nil {
			dropped++
		}
	}
	kept := 0
	for i := range s.rules {
		kept += len(s.rules[i].entries)
	}
	return kept, dropped
}
