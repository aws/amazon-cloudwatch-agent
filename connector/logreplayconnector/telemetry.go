// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"

	"github.com/aws/amazon-cloudwatch-agent/internal/statestore"
)

// The connector's own telemetry. The instruments are hand-written in the
// form that mdatagen generates for collector components:
// otelcol_<kind>_<type>_<name>, on a meter named after the package.
const (
	telemetryPrefix         = "otelcol_connector_log_replay_"
	metricRecordsMatched    = telemetryPrefix + "records_matched"
	metricRecordsNotUsed    = telemetryPrefix + "records_not_used"
	metricRecordsReplayed   = telemetryPrefix + "records_replayed"
	metricRecordsSkipped    = telemetryPrefix + "records_skipped"
	metricReplayErrors      = telemetryPrefix + "replay_errors"
	metricEntriesRemoved    = telemetryPrefix + "entries_removed"
	metricEntries           = telemetryPrefix + "entries"
	metricStateBytes        = telemetryPrefix + "state_bytes"
	metricStateSaves        = telemetryPrefix + "state_saves"
	metricStateSaveErrors   = telemetryPrefix + "state_save_errors"
	metricStateSaveRejected = telemetryPrefix + "state_save_rejected"
	metricStateSavedBytes   = telemetryPrefix + "state_saved_bytes"
)

// Attribute keys of the connector's own telemetry.
const (
	attrRule   = "rule"
	attrReason = "reason"
)

// notUsedReason is why a record that matched a rule was not kept.
type notUsedReason uint8

const (
	// notUsedIdentityMissing: an identity attribute of the resource is
	// missing or empty.
	notUsedIdentityMissing notUsedReason = iota
	// notUsedSourceFileMissing: source_file_attribute is set, and the
	// record does not have it.
	notUsedSourceFileMissing
	// notUsedTooLarge: the record is larger than max_record_bytes.
	notUsedTooLarge
	// notUsedEncodeError: the record cannot be encoded as OTLP JSON.
	notUsedEncodeError
	// notUsedEvaluationError: with error_mode propagate, an OTTL
	// evaluation error of a condition stopped the batch at this record.
	notUsedEvaluationError

	numNotUsedReasons
)

var notUsedReasonNames = [numNotUsedReasons]string{
	"identity_missing", "source_file_missing", "too_large", "encode_error", "evaluation_error",
}

var removeReasonNames = [numRemoveReasons]string{"source_gone", "max_entries", "max_state_bytes", "too_large"}

// skipReason is why a kept record was not sent at a tick. It stays kept.
type skipReason uint8

const (
	// skipSourceCheckError: the source file could not be checked, for a
	// reason other than that it does not exist.
	skipSourceCheckError skipReason = iota
	// skipDecodeError: the kept record cannot be decoded.
	skipDecodeError

	numSkipReasons
)

var skipReasonNames = [numSkipReasons]string{"source_check_error", "decode_error"}

// skip is a kept record that a tick did not send.
type skip struct {
	rule   int
	reason skipReason
}

// ruleCounts counts what one ConsumeLogs call did for one rule. The
// connector adds the counts to the instruments once per call.
type ruleCounts struct {
	matched int64
	notUsed [numNotUsedReasons]int64
}

// ruleAttrs holds the attribute options of one rule, built once, so that
// recording a measurement does not build an attribute set.
type ruleAttrs struct {
	rule    []otelmetric.AddOption
	notUsed [numNotUsedReasons][]otelmetric.AddOption
	removed [numRemoveReasons][]otelmetric.AddOption
	skipped [numSkipReasons][]otelmetric.AddOption
	observe []otelmetric.ObserveOption
}

// telemetry holds the instruments of the connector's own telemetry.
type telemetry struct {
	recordsMatched  otelmetric.Int64Counter
	recordsNotUsed  otelmetric.Int64Counter
	recordsReplayed otelmetric.Int64Counter
	recordsSkipped  otelmetric.Int64Counter
	replayErrors    otelmetric.Int64Counter
	entriesRemoved  otelmetric.Int64Counter
	entries         otelmetric.Int64ObservableGauge
	stateBytes      otelmetric.Int64ObservableGauge

	// rules is indexed like the connector rules.
	rules []ruleAttrs

	meter        otelmetric.Meter
	registration otelmetric.Registration
	// storeRegistration is the callback of the state store instruments. It
	// is set by registerStore, only when the state store opens.
	storeRegistration otelmetric.Registration
	unregister        sync.Once
}

// newTelemetry creates the instruments and registers the callback of the
// gauges, which reads s under its lock.
func newTelemetry(mp otelmetric.MeterProvider, ruleNames []string, s *state) (*telemetry, error) {
	meter := mp.Meter(scopeName)
	t := &telemetry{rules: make([]ruleAttrs, len(ruleNames)), meter: meter}
	var errs error
	newCounter := func(name, desc, unit string) otelmetric.Int64Counter {
		c, err := meter.Int64Counter(name, otelmetric.WithDescription(desc), otelmetric.WithUnit(unit))
		errs = errors.Join(errs, err)
		return c
	}
	newGauge := func(name, desc, unit string) otelmetric.Int64ObservableGauge {
		g, err := meter.Int64ObservableGauge(name, otelmetric.WithDescription(desc), otelmetric.WithUnit(unit))
		errs = errors.Join(errs, err)
		return g
	}
	t.recordsMatched = newCounter(metricRecordsMatched,
		"Number of log records that passed the body_contains check and the conditions of a rule.", "{records}")
	t.recordsNotUsed = newCounter(metricRecordsNotUsed,
		"Number of log records that matched a rule but that the connector did not keep.", "{records}")
	t.recordsReplayed = newCounter(metricRecordsReplayed,
		"Number of log record copies accepted by the next consumer.", "{records}")
	t.recordsSkipped = newCounter(metricRecordsSkipped,
		"Number of kept log records not sent at a tick. They stay kept for the next tick.", "{records}")
	t.replayErrors = newCounter(metricReplayErrors,
		"Number of calls to the next consumer that failed, counted once for each rule in the call.", "{errors}")
	t.entriesRemoved = newCounter(metricEntriesRemoved,
		"Number of kept log records removed because the source file is gone or because of a state limit, or not stored because of a state limit.", "{records}")
	t.entries = newGauge(metricEntries, "Number of kept log records.", "{records}")
	t.stateBytes = newGauge(metricStateBytes, "Size estimate of the serialized state.", "By")
	if errs != nil {
		return nil, errs
	}

	for i, name := range ruleNames {
		rule := attribute.String(attrRule, name)
		ra := &t.rules[i]
		ra.rule = addOptions(rule)
		for r := range numNotUsedReasons {
			ra.notUsed[r] = addOptions(rule, attribute.String(attrReason, notUsedReasonNames[r]))
		}
		for r := range numRemoveReasons {
			ra.removed[r] = addOptions(rule, attribute.String(attrReason, removeReasonNames[r]))
		}
		for r := range numSkipReasons {
			ra.skipped[r] = addOptions(rule, attribute.String(attrReason, skipReasonNames[r]))
		}
		ra.observe = []otelmetric.ObserveOption{otelmetric.WithAttributeSet(attribute.NewSet(rule))}
	}

	reg, err := meter.RegisterCallback(func(_ context.Context, o otelmetric.Observer) error {
		counts, bytes := s.sizes()
		for i, n := range counts {
			o.ObserveInt64(t.entries, int64(n), t.rules[i].observe...)
		}
		o.ObserveInt64(t.stateBytes, int64(bytes))
		return nil
	}, t.entries, t.stateBytes)
	if err != nil {
		return nil, err
	}
	t.registration = reg
	return t, nil
}

func addOptions(attrs ...attribute.KeyValue) []otelmetric.AddOption {
	return []otelmetric.AddOption{otelmetric.WithAttributeSet(attribute.NewSet(attrs...))}
}

// registerStore creates the instruments of the state store and registers
// their callback, which reads stats. The connector calls it in Start, only
// when storage is configured and the state store opens.
func (t *telemetry) registerStore(stats func() statestore.Stats) error {
	var errs error
	newCounter := func(name, desc, unit string) otelmetric.Int64ObservableCounter {
		c, err := t.meter.Int64ObservableCounter(name, otelmetric.WithDescription(desc), otelmetric.WithUnit(unit))
		errs = errors.Join(errs, err)
		return c
	}
	saves := newCounter(metricStateSaves,
		"Number of times the state was written to the storage extension.", "{saves}")
	saveErrors := newCounter(metricStateSaveErrors,
		"Number of state saves that failed in the snapshot, the encoding, or the storage write.", "{errors}")
	rejected := newCounter(metricStateSaveRejected,
		"Number of state saves refused because the record was larger than the limit.", "{saves}")
	savedBytes, err := t.meter.Int64ObservableGauge(metricStateSavedBytes,
		otelmetric.WithDescription("Size of the state record that was last loaded from or saved to the storage extension."),
		otelmetric.WithUnit("By"))
	errs = errors.Join(errs, err)
	if errs != nil {
		return errs
	}
	reg, err := t.meter.RegisterCallback(func(_ context.Context, o otelmetric.Observer) error {
		s := stats()
		o.ObserveInt64(saves, clampInt64(s.Saves))
		o.ObserveInt64(saveErrors, clampInt64(s.SaveErrors))
		o.ObserveInt64(rejected, clampInt64(s.Rejected))
		o.ObserveInt64(savedBytes, int64(s.LastBytes))
		return nil
	}, saves, saveErrors, rejected, savedBytes)
	if err != nil {
		return err
	}
	t.storeRegistration = reg
	return nil
}

// clampInt64 converts a counter to int64, with math.MaxInt64 for a value
// that does not fit.
func clampInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}

// shutdown unregisters the callbacks of the observable instruments. It is
// safe to call more than once.
func (t *telemetry) shutdown() {
	t.unregister.Do(func() {
		if t.registration != nil {
			_ = t.registration.Unregister()
		}
		if t.storeRegistration != nil {
			_ = t.storeRegistration.Unregister()
		}
	})
}

// recordCounts adds the counts of one ConsumeLogs call, indexed by rule.
func (t *telemetry) recordCounts(ctx context.Context, counts []ruleCounts) {
	for i := range counts {
		rc := &counts[i]
		ra := &t.rules[i]
		if rc.matched > 0 {
			t.recordsMatched.Add(ctx, rc.matched, ra.rule...)
		}
		for r := range numNotUsedReasons {
			if n := rc.notUsed[r]; n > 0 {
				t.recordsNotUsed.Add(ctx, n, ra.notUsed[r]...)
			}
		}
	}
}

// recordRemovals adds the removals of one batch or tick, under the rule of
// each removed entry. A new entry that was not stored because of a state
// limit counts as removed with the reason of that limit.
func (t *telemetry) recordRemovals(ctx context.Context, removed, rejected []removal) {
	if len(removed) == 0 && len(rejected) == 0 {
		return
	}
	counts := make([][numRemoveReasons]int64, len(t.rules))
	for _, r := range removed {
		counts[r.rule][r.reason]++
	}
	for _, r := range rejected {
		counts[r.rule][r.reason]++
	}
	for i := range counts {
		for r := range numRemoveReasons {
			if n := counts[i][r]; n > 0 {
				t.entriesRemoved.Add(ctx, n, t.rules[i].removed[r]...)
			}
		}
	}
}

// recordSkipped adds the kept records that a tick did not send.
func (t *telemetry) recordSkipped(ctx context.Context, skipped []skip) {
	for _, s := range skipped {
		t.recordsSkipped.Add(ctx, 1, t.rules[s.rule].skipped[s.reason]...)
	}
}

// recordReplay adds the result of one call to the next consumer: the copies
// of each rule when it succeeded, else one error for each rule in the call.
// sent is indexed by rule.
func (t *telemetry) recordReplay(ctx context.Context, sent []int64, err error) {
	for i, n := range sent {
		switch {
		case n == 0:
		case err != nil:
			t.replayErrors.Add(ctx, 1, t.rules[i].rule...)
		default:
			t.recordsReplayed.Add(ctx, n, t.rules[i].rule...)
		}
	}
}

// logInterval bounds how often each rate-limited log is written.
const logInterval = time.Minute

// logLimiter allows one log per logInterval and counts what it suppresses
// in between. It is safe for concurrent use.
type logLimiter struct {
	// last is the time (Unix nanoseconds) of the last allowed log, 0
	// before the first.
	last       atomic.Int64
	suppressed atomic.Int64
}

// allow reports whether a log is allowed at now (Unix nanoseconds). When it
// is, it also returns the count suppressed since the previous log and resets
// it. When it is not, it adds n to that count.
func (l *logLimiter) allow(now, n int64) (int64, bool) {
	last := l.last.Load()
	if last != 0 && now-last < logInterval.Nanoseconds() {
		l.suppressed.Add(n)
		return 0, false
	}
	if !l.last.CompareAndSwap(last, now) {
		l.suppressed.Add(n)
		return 0, false
	}
	return l.suppressed.Swap(0), true
}
