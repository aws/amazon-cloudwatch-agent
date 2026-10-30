// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/statestore"
)

// replayConnector keeps the latest log record that matches a rule for each
// entity, and sends a copy of every kept record at each interval.
type replayConnector struct {
	// id is the component ID, which names the storage client.
	id     component.ID
	logger *zap.Logger
	cfg    *Config
	next   consumer.Logs
	rules  []compiledRule
	state  *state
	tel    *telemetry
	// now returns the current time. Tests replace it.
	now func() time.Time
	// checkSource returns nil when a source file exists, and an error
	// that wraps fs.ErrNotExist when it does not. Tests replace it.
	checkSource func(path string) error
	// recheckDelay is the wait before a missing source file is checked a
	// second time. Tests replace it.
	recheckDelay time.Duration

	// startupDelay and jitter set the first tick: startupDelay plus
	// jitter(min(interval, maxStartupJitter)). jitter returns a duration in
	// [0, n). Tests replace them.
	startupDelay time.Duration
	jitter       func(n time.Duration) time.Duration
	// stopReplay and replayDone are set by Start. replayDone is closed when
	// the replay goroutine has stopped.
	stopReplay context.CancelFunc
	replayDone chan struct{}

	// changed is called after the kept records changed. It is a no-op
	// unless the state is persisted.
	changed func()
	// store persists the state. It is set by Start when storage is
	// configured and opens, and nil otherwise.
	store *statestore.Store[stateSnapshot]

	downstreamErrLog logLimiter
	sourceErrLog     logLimiter
	tooLargeLog      logLimiter
	// removalLog is indexed by removeReason. Only max_entries and
	// max_state_bytes are logged at Warn.
	removalLog [numRemoveReasons]logLimiter
}

var _ connector.Logs = (*replayConnector)(nil)

func newConnector(set connector.Settings, cfg *Config, next consumer.Logs) (*replayConnector, error) {
	rules, err := compileRules(cfg, set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	specs := make([]ruleSpec, len(cfg.Rules))
	names := make([]string, len(cfg.Rules))
	for i := range cfg.Rules {
		rc := &cfg.Rules[i]
		specs[i] = ruleSpec{name: rc.Name, identity: rc.Identity, maxEntries: rc.effectiveMaxEntries()}
		names[i] = rc.Name
	}
	st := newState(specs, cfg.MaxStateBytes)
	tel, err := newTelemetry(set.MeterProvider, names, st)
	if err != nil {
		return nil, fmt.Errorf("failed to create the connector telemetry: %w", err)
	}
	return &replayConnector{
		id:           set.ID,
		logger:       set.Logger,
		cfg:          cfg,
		next:         next,
		rules:        rules,
		state:        st,
		tel:          tel,
		now:          time.Now,
		checkSource:  lstatFile,
		recheckDelay: sourceRecheckDelay,
		startupDelay: startupReplayDelay,
		jitter:       randomJitter,
		changed:      func() {},
	}, nil
}

// Start restores the stored records when storage is configured, then
// starts the replay and returns. It does not send: the first tick comes
// after a startup delay, also for restored records. It returns an error
// only when the storage extension is missing or is not a storage
// extension. If the extension cannot give a storage client, Start logs an
// Error and the records are kept in memory only.
func (c *replayConnector) Start(ctx context.Context, host component.Host) error {
	c.logger.Debug("Starting log_replay connector", zap.Int("rules", len(c.rules)), zap.Duration("interval", c.cfg.Interval))
	if c.cfg.Storage != nil {
		if err := c.startPersistence(ctx, host); err != nil {
			return err
		}
	}
	c.startReplaying()
	return nil
}

// Shutdown cancels the replay, saves the state a last time and closes the
// storage client, then waits for the replay to stop, all bounded by ctx.
// The errors are joined. It is safe to call without Start and more than
// once.
//
// The final save does not wait for a tick in progress, so a tick stuck in
// the next consumer cannot use up the deadline before the save. This is
// safe: a tick changes the state only under its lock and never uses the
// store, and the collector shuts down the receivers of the logs pipeline
// before the connector, so no ConsumeLogs runs during Shutdown.
func (c *replayConnector) Shutdown(ctx context.Context) error {
	c.cancelReplaying()
	var err error
	if c.store != nil {
		err = c.store.Shutdown(ctx)
	}
	err = errors.Join(err, c.waitReplaying(ctx))
	c.tel.shutdown()
	return err
}

// Capabilities reports that the connector does not mutate its input. The
// connector shares a logs pipeline with other exporters, and MutatesData
// false lets the fanout pass the same data to all of them without a copy.
func (c *replayConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// batch is the per-call scratch space of ConsumeLogs. It is allocated only
// when a record passes the body_contains check of a rule.
type batch struct {
	// idents caches the identity of the current ResourceLogs, per rule.
	idents []identityInfo
	obs    []observation
	// counts is indexed by rule.
	counts []ruleCounts
}

// identityInfo is the identity of one ResourceLogs for one rule.
type identityInfo struct {
	resolved bool
	// ok is false when an identity attribute is missing or empty.
	ok       bool
	key      string
	identity []kv
}

// ConsumeLogs scans the records and keeps a copy of each record that
// matches a rule. It sends nothing: the logs pipeline that feeds the
// connector already exports the original record. It may be called
// concurrently. It returns an error only in propagate mode, for an OTTL
// evaluation error of a condition; the records matched before that record
// are still kept.
func (c *replayConnector) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var b *batch
	var err error
	rls := ld.ResourceLogs()
scan:
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		if b != nil {
			clear(b.idents)
		}
		sls := rl.ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			sl := sls.At(j)
			lrs := sl.LogRecords()
			for k := 0; k < lrs.Len(); k++ {
				lr := lrs.At(k)
				for ri := range c.rules {
					// The hot path: most records fail here, without allocating.
					if !c.rules[ri].matchesBody(lr) {
						continue
					}
					if b == nil {
						b = &batch{
							idents: make([]identityInfo, len(c.rules)),
							counts: make([]ruleCounts, len(c.rules)),
						}
					}
					if err = c.observe(ctx, b, ri, rl, sl, lr); err != nil {
						break scan
					}
				}
			}
		}
	}
	c.apply(ctx, b)
	return err
}

// observe evaluates a rule on a record that passed body_contains and, if
// the rule keeps the record, appends an observation to b. It counts the
// outcome in b.counts.
func (c *replayConnector) observe(ctx context.Context, b *batch, ri int, rl plog.ResourceLogs, sl plog.ScopeLogs, lr plog.LogRecord) error {
	rule := &c.rules[ri]
	counts := &b.counts[ri]

	if rule.conditions != nil {
		tCtx := ottllog.NewTransformContextPtr(rl, sl, lr)
		match, err := rule.matchesConditions(ctx, tCtx)
		tCtx.Close()
		if err != nil {
			counts.notUsed[notUsedEvaluationError]++
			return fmt.Errorf("rule %q: %w", rule.cfg.Name, err)
		}
		if !match {
			return nil
		}
	}
	counts.matched++

	id := &b.idents[ri]
	if !id.resolved {
		id.identity, id.key, id.ok = rule.resolveIdentity(rl.Resource().Attributes())
		id.resolved = true
		if !id.ok {
			if ce := c.logger.Check(zap.DebugLevel, "Matching log record not kept: an identity attribute is missing or empty"); ce != nil {
				ce.Write(zap.String("rule", rule.cfg.Name), zap.Strings("identity", rule.cfg.Identity))
			}
		}
	}
	if !id.ok {
		counts.notUsed[notUsedIdentityMissing]++
		return nil
	}

	src, ok := sourceFile(lr, c.cfg.SourceFileAttribute)
	if !ok {
		counts.notUsed[notUsedSourceFileMissing]++
		if ce := c.logger.Check(zap.DebugLevel, "Matching log record not kept: the source file attribute is missing or empty"); ce != nil {
			ce.Write(zap.String("rule", rule.cfg.Name), zap.String("source_file_attribute", c.cfg.SourceFileAttribute))
		}
		return nil
	}

	data, err := encodeRecord(rl, sl, lr)
	if err != nil {
		counts.notUsed[notUsedEncodeError]++
		if ce := c.logger.Check(zap.DebugLevel, "Matching log record not kept: it cannot be encoded"); ce != nil {
			ce.Write(zap.String("rule", rule.cfg.Name), zap.Error(err))
		}
		return nil
	}
	if len(data) > c.cfg.MaxRecordBytes {
		counts.notUsed[notUsedTooLarge]++
		c.logTooLarge(rule.cfg.Name, len(data))
		// The state removes a kept record of the entity that is not newer.
		b.obs = append(b.obs, observation{rule: ri, key: id.key, time: c.recordTime(lr), tooLarge: true})
		return nil
	}

	b.obs = append(b.obs, observation{
		rule:       ri,
		key:        id.key,
		identity:   id.identity,
		record:     data,
		sourceFile: src,
		time:       c.recordTime(lr),
	})
	return nil
}

// apply adds the batch counts to the telemetry and applies the
// observations. The state lock is released before the hook.
func (c *replayConnector) apply(ctx context.Context, b *batch) {
	if b == nil {
		return
	}
	c.tel.recordCounts(ctx, b.counts)
	if len(b.obs) == 0 {
		return
	}
	res := c.state.applyBatch(b.obs)
	if res.changed() {
		c.changed()
	}
	c.tel.recordRemovals(ctx, res.removed, res.rejected)
	c.logRemovals(res.removed, res.rejected)
}

// logRemovals logs a removal for max_entries or max_state_bytes at Warn, at
// most once per logInterval for each of the two. The removed entries are
// the oldest ones, so the limit is too small only if their sources still
// run. Other removals, and the ones in between, are logged at Debug. The
// rejected entries count as removed.
func (c *replayConnector) logRemovals(removed, rejected []removal) {
	if len(removed) == 0 && len(rejected) == 0 {
		return
	}
	var counts [numRemoveReasons]int64
	var firstRule [numRemoveReasons]int
	for _, r := range slices.Concat(removed, rejected) {
		if counts[r.reason] == 0 {
			firstRule[r.reason] = r.rule
		}
		counts[r.reason]++
	}
	var now int64
	for _, reason := range [...]removeReason{removeMaxEntries, removeMaxStateBytes} {
		n := counts[reason]
		if n == 0 {
			continue
		}
		if now == 0 {
			now = c.now().UnixNano()
		}
		suppressed, ok := c.removalLog[reason].allow(now, n)
		if !ok {
			continue
		}
		c.logger.Warn("Removed the oldest kept log records to stay within the limit. The limit is too small only if their sources still run.",
			zap.String("limit", removeReasonNames[reason]),
			zap.String("rule", c.rules[firstRule[reason]].cfg.Name),
			zap.Int64("removed", n),
			zap.Int64("suppressed_since_last_log", suppressed),
		)
		counts[reason] = 0
	}
	if counts == ([numRemoveReasons]int64{}) {
		return
	}
	if ce := c.logger.Check(zap.DebugLevel, "Removed kept log records"); ce != nil {
		ce.Write(
			zap.Int64("source_gone", counts[removeSourceGone]),
			zap.Int64("max_entries", counts[removeMaxEntries]),
			zap.Int64("max_state_bytes", counts[removeMaxStateBytes]),
			zap.Int64("too_large", counts[removeTooLarge]),
		)
	}
}

// logTooLarge logs a matching record that was not kept because it is larger
// than max_record_bytes, at Warn at most once per logInterval, and at Debug
// in between.
func (c *replayConnector) logTooLarge(rule string, size int) {
	const msg = "Matching log record not kept: it is larger than max_record_bytes"
	if suppressed, ok := c.tooLargeLog.allow(c.now().UnixNano(), 1); ok {
		c.logger.Warn(msg,
			zap.String("rule", rule),
			zap.Int("bytes", size),
			zap.Int("max_record_bytes", c.cfg.MaxRecordBytes),
			zap.Int64("suppressed_since_last_log", suppressed),
		)
		return
	}
	if ce := c.logger.Check(zap.DebugLevel, msg); ce != nil {
		ce.Write(zap.String("rule", rule), zap.Int("bytes", size))
	}
}

// recordTime returns the record's timestamp, else its observed timestamp,
// else now, in Unix nanoseconds. A time ahead of now is now, so that a
// record with a skewed clock cannot keep an entry from eviction.
func (c *replayConnector) recordTime(lr plog.LogRecord) int64 {
	now := c.now().UnixNano()
	if ts := recordTimestamp(lr); ts != 0 {
		return min(ts.AsTime().UnixNano(), now)
	}
	return now
}
