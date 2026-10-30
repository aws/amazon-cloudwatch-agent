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

// Size estimate of the serialized state, used for max_state_bytes. The
// estimate of one entry is the sum of the JSON-encoded lengths of its state
// key, its identity, its record, and its source file, plus these constants.
// They cover the rest of the JSON encoding of an entry in a snapshot, with
// the comma that separates it from the next, so the estimate is at least the
// encoded size.
const (
	// entryOverheadBytes covers the field names, brackets, and separators
	// of one entry, and its time at up to 20 characters:
	// "<key>":{"identity":[],"record":,"source_file":"","time":N},
	entryOverheadBytes = 96
	// pairOverheadBytes covers the framing of one key/value pair:
	// {"key":"","value":""},
	pairOverheadBytes = 24
)

// kv is one key/value pair. Lists of pairs keep the configured order.
type kv struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// entry is the latest matching record of one identity of one rule. The
// state never modifies a slice of an entry after storing it; a change
// stores new slices. Copies of an entry can therefore share its slices, and
// a snapshot can be encoded without holding the state lock.
type entry struct {
	// Identity holds the identity keys and values, in the rule's order.
	Identity []kv `json:"identity"`
	// Record is the kept record with its resource and scope, as compact
	// OTLP JSON in which encoding/json has nothing left to escape, so that
	// its length is its encoded length. See encodeRecord.
	Record json.RawMessage `json:"record"`
	// SourceFile is the value of the source file attribute of the record,
	// or empty when source_file_attribute is not set.
	SourceFile string `json:"source_file,omitempty"`
	// Time is the record time in Unix nanoseconds. A record older than Time
	// does not replace the entry, and the oldest entries are evicted first.
	Time int64 `json:"time"`
}

// observation is a matching record, ready to be applied to the state.
type observation struct {
	rule       int
	key        string
	identity   []kv
	record     []byte
	sourceFile string
	// time is the record time in Unix nanoseconds.
	time int64
	// tooLarge marks a record larger than max_record_bytes. It has no
	// record, and it removes a kept entry that is not newer, because that
	// entry is stale.
	tooLarge bool
}

// outcome is the result of applying one observation.
type outcome uint8

const (
	// outcomeInserted: a new entry was stored.
	outcomeInserted outcome = iota
	// outcomeReplaced: a record at least as recent replaced the entry.
	outcomeReplaced
	// outcomeUnchanged: the same record was seen again.
	outcomeUnchanged
	// outcomeIgnoredOlder: the record is older than the entry and was not
	// used.
	outcomeIgnoredOlder
	// outcomeTooLarge: the record is larger than max_record_bytes, or the
	// entry alone exceeds max_state_bytes, and it was not stored. If an
	// entry that is not newer existed, it was removed, because it was stale.
	outcomeTooLarge
	// outcomeOverLimit: the rule keeps max_entries records, and the record
	// is older than all of them, so it was not stored.
	outcomeOverLimit

	numOutcomes
)

// entryRef identifies an entry in the state.
type entryRef struct {
	rule int
	key  string
}

// removeReason is why an entry left the state.
type removeReason uint8

const (
	// removeSourceGone: the source file of the record no longer exists.
	removeSourceGone removeReason = iota
	// removeMaxEntries: a new entry of the same rule needed room, or a new
	// entry was older than every entry of a full rule.
	removeMaxEntries
	// removeMaxStateBytes: the state was over max_state_bytes.
	removeMaxStateBytes
	// removeTooLarge: a new record of the entry is too large to keep.
	removeTooLarge

	numRemoveReasons
)

// removal is an entry removed from the state.
type removal struct {
	entryRef
	reason removeReason
}

// batchResult summarizes applyBatch.
type batchResult struct {
	// counts is indexed by outcome.
	counts  [numOutcomes]int
	removed []removal
	// rejected lists the new entries that were not stored because of a
	// state limit: max_state_bytes (reason too_large) or max_entries.
	rejected []removal
}

// changed reports whether the batch changed the stored records.
func (r *batchResult) changed() bool {
	return r.counts[outcomeInserted] > 0 || r.counts[outcomeReplaced] > 0 || len(r.removed) > 0
}

// applyResult is the result of applying one observation.
type applyResult struct {
	outcome outcome
	removed []removal
	// rejected is set when a new entry was not stored because of a limit.
	rejected *removal
}

// ruleSpec is what the state needs to know about a rule.
type ruleSpec struct {
	name       string
	identity   []string
	maxEntries int
}

type storedEntry struct {
	entry
	// size is the estimate of the entry's serialized size.
	size int
}

type ruleState struct {
	ruleSpec
	entries map[string]*storedEntry
}

// state holds the entries of every rule. It does not use pdata types. One
// mutex protects it; callers prepare observations before taking the lock.
type state struct {
	mu            sync.Mutex
	rules         []ruleState
	ruleIndex     map[string]int
	maxStateBytes int
	// bytes is the sum of the size estimates of all entries.
	bytes int
	// lastReplay is the time (Unix nanoseconds) of the latest tick whose
	// copies the next consumer accepted, 0 if none.
	lastReplay int64
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

// applyBatch applies the observations in order, under one lock.
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
		// The record itself is not kept. A kept record that is not newer is
		// stale, and sending it would show a configuration that is gone.
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

// insertLocked stores a new entry, evicting the rule's oldest entry at
// max_entries, then the oldest entries of any rule while the state is over
// max_state_bytes. At max_entries, a new entry that is older than every
// entry of the rule is not stored, because the rule keeps the latest
// records.
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

// evictForBytesLocked evicts the oldest entries of any rule, never the entry
// (keepRule, keepKey), until the state fits max_state_bytes.
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

// oldestLocked returns the entry with the smallest record time, ties broken
// by the smallest rule name and then the smallest key. It scans one rule, or
// all rules when rule is -1, and skips the entry (skipRule, skipKey) when
// skipKey is not empty. The scan is O(n), which is fine for the configured
// limits because eviction is rare.
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

// keptEntry is a copy of an entry, with its place in the state.
type keptEntry struct {
	entryRef
	entry entry
}

// entries returns copies of all entries, sorted by rule index and then by
// key. The copies share their slices with the state, which never modifies
// them, so the caller can use them without the lock.
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

// removeGone removes the entries whose source file is gone. It removes an
// entry only if it still holds the record that was checked, so a record
// that replaced it during the check stays. It returns the removals.
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

// sizes returns the number of entries of each rule, indexed by rule, and
// the size estimate of the state.
func (s *state) sizes() ([]int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := make([]int, len(s.rules))
	for i, rs := range s.rules {
		counts[i] = len(rs.entries)
	}
	return counts, s.bytes
}

// entrySize estimates the serialized size of an entry; see
// entryOverheadBytes. The record is stored in its encoded form, so its
// length counts as is.
func entrySize(key string, e *entry) int {
	n := entryOverheadBytes + jsonStringLen(key) + len(e.Record) + jsonStringLen(e.SourceFile)
	for _, p := range e.Identity {
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
// identity key, then entry. Rules without entries are omitted. LastReplay
// is the time (Unix nanoseconds) of the latest copies, so that a restart
// does not send them again before the interval ends.
type stateSnapshot struct {
	Rules      map[string]map[string]entry `json:"rules"`
	LastReplay int64                       `json:"last_replay,omitempty"`
}

// setLastReplay stores the time of the latest copies.
func (s *state) setLastReplay(t int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastReplay = t
}

// lastReplayTime returns the time of the latest copies, 0 if none.
func (s *state) lastReplayTime() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastReplay
}

// snapshot returns a copy of the state. The entries share their slices with
// the state, which never modifies them, so the snapshot can be encoded
// without the lock.
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

// restoredEntry is an entry of a snapshot that the restore keeps.
type restoredEntry struct {
	entryRef
	entry entry
}

// restore replaces the state with the given entries and the time of the
// latest copies, and returns how many entries it kept and how many the
// limits dropped. It enforces max_entries and max_state_bytes, keeping the
// most recent records. The state takes ownership of the entries' slices.
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
