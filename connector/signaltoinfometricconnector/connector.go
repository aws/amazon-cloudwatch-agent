// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/statestore"
)

// infoMetricConnector matches log records against the rules, keeps the
// latest facts per rule and entity, and emits an Info metric when the facts
// of an entity change and at each emit interval.
type infoMetricConnector struct {
	// id is the component ID, which names the storage client.
	id     component.ID
	logger *zap.Logger
	cfg    *Config
	next   consumer.Metrics
	rules  []compiledRule
	state  *state
	tel    *telemetry
	// now returns the current time. Tests replace it.
	now func() time.Time

	// groups are the emit groups, one per distinct effective interval.
	groups []emitGroup
	// startupDelay and jitter set the first tick of each group:
	// startupDelay plus jitter(min(interval, maxStartupJitter)). jitter
	// returns a duration in [0, n). Tests replace them.
	startupDelay time.Duration
	jitter       func(n time.Duration) time.Duration
	// stopEmit and emitDone are set by Start. emitDone is closed when every
	// group has stopped.
	stopEmit context.CancelFunc
	emitDone chan struct{}

	// hooks are called after ConsumeLogs changed the state.
	hooks stateHooks
	// store persists the state. It is set by Start when storage is
	// configured and opens, and nil otherwise.
	store *statestore.Store[stateSnapshot]

	downstreamErrLog logLimiter
	// evictionLog is indexed by evictReason. Only max_entities and
	// max_state_bytes are logged at Warn.
	evictionLog [numEvictReasons]logLimiter
	// requiredMissingLog is indexed by rule.
	requiredMissingLog []logLimiter
}

// stateHooks let the persistence of the state learn about changes. They are
// called outside the state lock, after the batch that caused them. Both are
// no-ops by default.
type stateHooks struct {
	// changed is called when a batch inserted, changed, or evicted an
	// entity, or found an entity too large to store.
	changed func()
	// touched is called when a batch moved the Activity or Updated time of
	// an existing entity forward. A batch can call both.
	touched func()
}

func noop() {}

var _ connector.Logs = (*infoMetricConnector)(nil)

func newConnector(set connector.Settings, cfg *Config, next consumer.Metrics) (*infoMetricConnector, error) {
	rules, err := compileRules(cfg, set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	specs := make([]ruleSpec, len(cfg.Rules))
	names := make([]string, len(cfg.Rules))
	for i := range cfg.Rules {
		rc := &cfg.Rules[i]
		specs[i] = ruleSpec{name: rc.Name, identity: rc.Identity, maxEntities: rc.effectiveMaxEntities()}
		names[i] = rc.Name
	}
	st := newState(specs, cfg.MaxStateBytes)
	tel, err := newTelemetry(set.MeterProvider, names, st)
	if err != nil {
		return nil, fmt.Errorf("failed to create the connector telemetry: %w", err)
	}
	return &infoMetricConnector{
		id:                 set.ID,
		logger:             set.Logger,
		cfg:                cfg,
		next:               next,
		rules:              rules,
		state:              st,
		tel:                tel,
		now:                time.Now,
		groups:             newEmitGroups(cfg),
		startupDelay:       startupEmitDelay,
		jitter:             randomJitter,
		hooks:              stateHooks{changed: noop, touched: noop},
		requiredMissingLog: make([]logLimiter, len(rules)),
	}, nil
}

// Start restores the stored state when storage is configured, then
// launches the periodic emission and returns. It does not emit: the first
// tick of each emit group comes after a startup delay, also for restored
// entities. It returns an error only when the storage extension is missing
// or is not a storage extension. If the extension cannot give a storage
// client, Start logs an Error and the state is in memory only.
func (c *infoMetricConnector) Start(ctx context.Context, host component.Host) error {
	c.logger.Debug("Starting signal_to_info_metric connector", zap.Int("rules", len(c.rules)), zap.Int("emit_groups", len(c.groups)))
	if c.cfg.Storage != nil {
		if err := c.startPersistence(ctx, host); err != nil {
			return err
		}
	}
	c.startEmitting()
	return nil
}

// Shutdown cancels the periodic emission, saves the state a last time and
// closes the storage client, then waits for the emission to stop, all
// bounded by ctx. The errors are joined. It is safe to call without Start
// and more than once.
//
// The final save does not wait for a tick in progress, so a tick stuck in
// the next consumer cannot use up the deadline before the save. This is
// safe: a tick only reads the state under its lock and never uses the
// store, and the collector shuts down the receivers of the logs pipeline
// before the connector, so no ConsumeLogs runs during Shutdown.
func (c *infoMetricConnector) Shutdown(ctx context.Context) error {
	c.cancelEmitting()
	var err error
	if c.store != nil {
		err = c.store.Shutdown(ctx)
	}
	err = errors.Join(err, c.waitEmitting(ctx))
	c.tel.shutdown()
	return err
}

// Capabilities reports that the connector does not mutate its input. The
// connector shares a logs pipeline with other exporters, and MutatesData
// false lets the fanout pass the same data to all of them without a copy.
func (c *infoMetricConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// batch is the per-call scratch space of ConsumeLogs. It is allocated only
// when a record passes the body_contains check of a rule.
type batch struct {
	// resources caches the resourceInfo of the current ResourceLogs, per
	// rule.
	resources []resourceInfo
	obs       []observation
	// counts is indexed by rule.
	counts []ruleCounts
}

// ConsumeLogs scans the records, then applies the activity and the
// observations to the state under one lock, then, with emit_on_change,
// emits the entities that were inserted or changed. It may be called
// concurrently. It returns an error only in propagate mode, for an OTTL
// evaluation error of a condition or of a required attribute without a
// default; the observations made before that record are still applied.
//
// When a rule has entities, every ResourceLogs whose identity resolves also
// records its latest record time, so that any record of a tracked entity,
// not only a matching one, keeps the entity from eviction. This path does
// not allocate once its pooled buffers have grown.
func (c *infoMetricConnector) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var b *batch
	var act *activityBatch
	if c.state.tracking() {
		act = getActivityBatch()
	}
	var err error
	rls := ld.ResourceLogs()
scan:
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		if b != nil {
			clear(b.resources)
		}
		// latest is the latest record timestamp of the ResourceLogs, and
		// untimed is set when a record has none, which means now.
		var latest pcommon.Timestamp
		untimed := false
		sls := rl.ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			sl := sls.At(j)
			lrs := sl.LogRecords()
			for k := 0; k < lrs.Len(); k++ {
				lr := lrs.At(k)
				if act != nil {
					if ts := recordTimestamp(lr); ts == 0 {
						untimed = true
					} else {
						latest = max(latest, ts)
					}
				}
				for ri := range c.rules {
					// The hot path: most records fail here, without allocating.
					if !c.rules[ri].matchesBody(lr) {
						continue
					}
					if b == nil {
						b = &batch{
							resources: make([]resourceInfo, len(c.rules)),
							counts:    make([]ruleCounts, len(c.rules)),
						}
					}
					if err = c.observe(ctx, b, ri, rl, sl, lr); err != nil {
						break scan
					}
				}
			}
		}
		if act != nil && (latest != 0 || untimed) {
			c.collectActivity(act, rl.Resource().Attributes(), latest, untimed)
		}
	}
	c.apply(ctx, b, act)
	return err
}

// collectActivity adds a touch for each rule that has entities and whose
// identity resolves on the resource. The touch time is the latest record
// time of the ResourceLogs, at most now.
func (c *infoMetricConnector) collectActivity(act *activityBatch, attrs pcommon.Map, latest pcommon.Timestamp, untimed bool) {
	if act.now == 0 {
		act.now = c.now().UnixNano()
	}
	t := act.now
	if !untimed {
		t = min(latest.AsTime().UnixNano(), act.now)
	}
	for ri := range c.rules {
		if !c.state.ruleTracked(ri) {
			continue
		}
		from := len(act.keys)
		keys, ok := c.rules[ri].appendIdentityKey(act.keys, attrs, c.cfg.MaxValueLength)
		if !ok {
			act.keys = keys[:from]
			continue
		}
		act.keys = keys
		act.touches = append(act.touches, touch{rule: ri, from: from, to: len(keys), time: t})
	}
}

// observe evaluates a rule on a record that passed body_contains and, if
// the record yields an entity, appends an observation to b. It counts the
// outcome in b.counts.
func (c *infoMetricConnector) observe(ctx context.Context, b *batch, ri int, rl plog.ResourceLogs, sl plog.ScopeLogs, lr plog.LogRecord) error {
	rule := &c.rules[ri]
	res := &b.resources[ri]
	counts := &b.counts[ri]

	tCtx := ottllog.NewTransformContextPtr(rl, sl, lr)
	defer tCtx.Close()
	match, err := rule.matchesConditions(ctx, tCtx)
	if err != nil {
		counts.notUsed[notUsedEvaluationError]++
		return fmt.Errorf("rule %q: %w", rule.cfg.Name, err)
	}
	if !match {
		return nil
	}
	counts.matched++

	if !res.resolved {
		*res = rule.resolveResource(rl.Resource().Attributes(), c.cfg.MaxValueLength)
		counts.truncated += int64(res.truncated)
		if !res.ok {
			if ce := c.logger.Check(zap.DebugLevel, "Matching log record not used: an identity attribute is missing or empty"); ce != nil {
				ce.Write(zap.String("rule", rule.cfg.Name), zap.Strings("identity", rule.cfg.Identity))
			}
		}
	}
	if !res.ok {
		counts.notUsed[notUsedIdentityMissing]++
		return nil
	}

	f, err := rule.extractFacts(ctx, tCtx, c.cfg.MaxValueLength, c.cfg.ErrorMode)
	counts.truncated += int64(f.truncated)
	if err != nil {
		counts.notUsed[notUsedEvaluationError]++
		return err
	}
	if f.missing != "" {
		counts.notUsed[notUsedRequiredMissing]++
		c.logRequiredMissing(ri, f.missing, f.missingErr)
		return nil
	}
	b.obs = append(b.obs, observation{
		rule:     ri,
		key:      res.key,
		identity: res.identity,
		resource: res.resource,
		facts:    f.values,
		time:     c.recordTime(lr),
	})
	return nil
}

// apply adds the batch counts to the telemetry, applies the activity and
// the observations, calls the hooks, and, with emit_on_change, emits the
// entities that were inserted or changed. The state lock is released
// before the hooks and the emission.
func (c *infoMetricConnector) apply(ctx context.Context, b *batch, act *activityBatch) {
	var obs []observation
	if b != nil {
		c.tel.recordCounts(ctx, b.counts)
		obs = b.obs
	}
	if act != nil && len(act.touches) == 0 {
		putActivityBatch(act)
		act = nil
	}
	if len(obs) == 0 && act == nil {
		return
	}
	res := c.state.applyBatch(obs, act, c.cfg.EmitOnChange)
	putActivityBatch(act)

	if res.stateChanged() {
		c.hooks.changed()
	}
	if res.touched {
		c.hooks.touched()
	}
	c.tel.recordEvictions(ctx, res.evicted, res.rejected)
	c.logEvictions(&res)
	if c.cfg.EmitOnChange {
		c.emit(ctx, res.changed, triggerChange)
	}
}

// logEvictions logs an eviction for max_entities or max_state_bytes at
// Warn, at most once per logInterval for each of the two. The evicted
// entities are the least active ones, often of deleted pods, so the limit
// is too small only if they were still active. Other evictions, and the
// ones in between, are logged at Debug.
func (c *infoMetricConnector) logEvictions(res *batchResult) {
	if len(res.evicted) == 0 && len(res.rejected) == 0 {
		return
	}
	var counts [numEvictReasons]int64
	var firstRule [numEvictReasons]int
	for _, e := range res.evicted {
		if counts[e.reason] == 0 {
			firstRule[e.reason] = e.rule
		}
		counts[e.reason]++
	}
	counts[evictTooLarge] += int64(len(res.rejected))
	var now int64
	for _, reason := range [...]evictReason{evictMaxEntities, evictMaxStateBytes} {
		n := counts[reason]
		if n == 0 {
			continue
		}
		if now == 0 {
			now = c.now().UnixNano()
		}
		suppressed, ok := c.evictionLog[reason].allow(now, n)
		if !ok {
			continue
		}
		c.logger.Warn("Evicted the least active Info metric entities to stay within the limit. The limit is too small only if the evicted entities were still active.",
			zap.String("limit", evictReasonNames[reason]),
			zap.String("rule", c.rules[firstRule[reason]].cfg.Name),
			zap.Int64("evicted", n),
			zap.Int64("suppressed_since_last_log", suppressed),
		)
		counts[reason] = 0
	}
	if counts == ([numEvictReasons]int64{}) {
		return
	}
	if ce := c.logger.Check(zap.DebugLevel, "Evicted Info metric entities"); ce != nil {
		ce.Write(
			zap.Int64("max_entities", counts[evictMaxEntities]),
			zap.Int64("max_state_bytes", counts[evictMaxStateBytes]),
			zap.Int64("too_large", counts[evictTooLarge]),
		)
	}
}

// logRequiredMissing logs a matching record that was not used because a
// required attribute has no value. It logs at Info at most once per
// logInterval for each rule, because a missing fact is the visible sign of
// a change of the log format, and at Debug in between. error_mode silent
// turns both off.
func (c *infoMetricConnector) logRequiredMissing(ri int, attribute string, err error) {
	if c.cfg.ErrorMode == ottl.SilentError {
		return
	}
	const msg = "Matching log record not used: a required attribute has no value"
	rule := c.rules[ri].cfg.Name
	if suppressed, ok := c.requiredMissingLog[ri].allow(c.now().UnixNano(), 1); ok {
		c.logger.Info(msg,
			zap.String("rule", rule),
			zap.String("attribute", attribute),
			zap.Error(err),
			zap.Int64("suppressed_since_last_log", suppressed),
		)
		return
	}
	if ce := c.logger.Check(zap.DebugLevel, msg); ce != nil {
		ce.Write(zap.String("rule", rule), zap.String("attribute", attribute), zap.Error(err))
	}
}

// recordTimestamp returns the record's timestamp, else its observed
// timestamp, else 0.
func recordTimestamp(lr plog.LogRecord) pcommon.Timestamp {
	if ts := lr.Timestamp(); ts != 0 {
		return ts
	}
	return lr.ObservedTimestamp()
}

// recordTime returns the record's timestamp, else its observed timestamp,
// else now, in Unix nanoseconds. A time ahead of now is now, so that a
// record with a skewed clock cannot keep an entity from eviction.
func (c *infoMetricConnector) recordTime(lr plog.LogRecord) int64 {
	now := c.now().UnixNano()
	if ts := recordTimestamp(lr); ts != 0 {
		return min(ts.AsTime().UnixNano(), now)
	}
	return now
}
