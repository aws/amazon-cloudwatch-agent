// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

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
	telemetryPrefix         = "otelcol_connector_signal_to_info_metric_"
	metricRecordsMatched    = telemetryPrefix + "records_matched"
	metricRecordsNotUsed    = telemetryPrefix + "records_not_used"
	metricValuesTruncated   = telemetryPrefix + "values_truncated"
	metricEntitiesEvicted   = telemetryPrefix + "entities_evicted"
	metricDatapointsEmitted = telemetryPrefix + "datapoints_emitted"
	metricEmitErrors        = telemetryPrefix + "emit_errors"
	metricEntities          = telemetryPrefix + "entities"
	metricStateBytes        = telemetryPrefix + "state_bytes"
	metricStateSaves        = telemetryPrefix + "state_saves"
	metricStateSaveErrors   = telemetryPrefix + "state_save_errors"
	metricStateSaveRejected = telemetryPrefix + "state_save_rejected"
	metricStateSavedBytes   = telemetryPrefix + "state_saved_bytes"
)

// Attribute keys of the connector's own telemetry.
const (
	attrRule    = "rule"
	attrReason  = "reason"
	attrTrigger = "trigger"
)

// notUsedReason is why a record that passed body_contains was not used.
type notUsedReason uint8

const (
	// notUsedIdentityMissing: an identity attribute of the resource is
	// missing or empty.
	notUsedIdentityMissing notUsedReason = iota
	// notUsedRequiredMissing: a required attribute has no value, and no
	// error was returned.
	notUsedRequiredMissing
	// notUsedEvaluationError: with error_mode propagate, an OTTL
	// evaluation error stopped the batch at this record.
	notUsedEvaluationError

	numNotUsedReasons
)

var notUsedReasonNames = [numNotUsedReasons]string{"identity_missing", "required_missing", "evaluation_error"}

var evictReasonNames = [numEvictReasons]string{"max_entities", "max_state_bytes", "too_large"}

var triggerNames = [numTriggers]string{"change", "interval"}

// ruleCounts counts what one ConsumeLogs call did for one rule. The
// connector adds the counts to the instruments once per call.
type ruleCounts struct {
	matched   int64
	notUsed   [numNotUsedReasons]int64
	truncated int64
}

// ruleAttrs holds the attribute options of one rule, built once, so that
// recording a measurement does not build an attribute set.
type ruleAttrs struct {
	rule    []otelmetric.AddOption
	notUsed [numNotUsedReasons][]otelmetric.AddOption
	evicted [numEvictReasons][]otelmetric.AddOption
	emitted [numTriggers][]otelmetric.AddOption
	observe []otelmetric.ObserveOption
}

// telemetry holds the instruments of the connector's own telemetry.
type telemetry struct {
	recordsMatched    otelmetric.Int64Counter
	recordsNotUsed    otelmetric.Int64Counter
	valuesTruncated   otelmetric.Int64Counter
	entitiesEvicted   otelmetric.Int64Counter
	datapointsEmitted otelmetric.Int64Counter
	emitErrors        otelmetric.Int64Counter
	entities          otelmetric.Int64ObservableGauge
	stateBytes        otelmetric.Int64ObservableGauge

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
		"Number of log records that passed the body_contains check of a rule but that the rule did not use. Records that fail the conditions are not counted.", "{records}")
	t.valuesTruncated = newCounter(metricValuesTruncated,
		"Number of values cut to max_value_length.", "{values}")
	t.entitiesEvicted = newCounter(metricEntitiesEvicted,
		"Number of entities removed from the state or not stored because of a state limit.", "{entities}")
	t.datapointsEmitted = newCounter(metricDatapointsEmitted,
		"Number of Info metric data points accepted by the next consumer.", "{datapoints}")
	t.emitErrors = newCounter(metricEmitErrors,
		"Number of calls to the next consumer that failed, counted once for each rule in the call.", "{errors}")
	t.entities = newGauge(metricEntities, "Number of entities in the state.", "{entities}")
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
		for r := range numEvictReasons {
			ra.evicted[r] = addOptions(rule, attribute.String(attrReason, evictReasonNames[r]))
		}
		for tr := range numTriggers {
			ra.emitted[tr] = addOptions(rule, attribute.String(attrTrigger, triggerNames[tr]))
		}
		ra.observe = []otelmetric.ObserveOption{otelmetric.WithAttributeSet(attribute.NewSet(rule))}
	}

	reg, err := meter.RegisterCallback(func(_ context.Context, o otelmetric.Observer) error {
		counts := make([]int, len(t.rules))
		bytes := s.sizes(counts)
		for i, n := range counts {
			o.ObserveInt64(t.entities, int64(n), t.rules[i].observe...)
		}
		o.ObserveInt64(t.stateBytes, int64(bytes))
		return nil
	}, t.entities, t.stateBytes)
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
		if rc.truncated > 0 {
			t.valuesTruncated.Add(ctx, rc.truncated, ra.rule...)
		}
	}
}

// recordEvictions adds the evictions of one batch, under the rule of each
// evicted entity. A new entity that was too large to store counts as
// evicted with the reason too_large.
func (t *telemetry) recordEvictions(ctx context.Context, evicted []eviction, rejected []entityRef) {
	if len(evicted) == 0 && len(rejected) == 0 {
		return
	}
	counts := make([][numEvictReasons]int64, len(t.rules))
	for _, e := range evicted {
		counts[e.rule][e.reason]++
	}
	for _, r := range rejected {
		counts[r.rule][evictTooLarge]++
	}
	for i := range counts {
		for r := range numEvictReasons {
			if n := counts[i][r]; n > 0 {
				t.entitiesEvicted.Add(ctx, n, t.rules[i].evicted[r]...)
			}
		}
	}
}

// recordEmit adds the result of one call to the next consumer: the data
// points of each rule when it succeeded, else one error for each rule in
// the call.
func (t *telemetry) recordEmit(ctx context.Context, entries []emitEntry, trigger emitTrigger, err error) {
	counts := make([]int64, len(t.rules))
	for i := range entries {
		counts[entries[i].rule]++
	}
	for i, n := range counts {
		switch {
		case n == 0:
		case err != nil:
			t.emitErrors.Add(ctx, 1, t.rules[i].rule...)
		default:
			t.datapointsEmitted.Add(ctx, n, t.rules[i].emitted[trigger]...)
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
