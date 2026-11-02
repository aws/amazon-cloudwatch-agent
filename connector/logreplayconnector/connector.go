// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type replayConnector struct {
	id     component.ID
	logger *zap.Logger
	cfg    *Config
	next   consumer.Logs
	rules  []compiledRule
	state  *state
	// Tests replace now, checkSource, recheckDelay, startupDelay, and jitter.
	now          func() time.Time
	checkSource  func(path string) error
	recheckDelay time.Duration

	startupDelay time.Duration
	jitter       func(n time.Duration) time.Duration
	stopReplay   context.CancelFunc
	replayDone   chan struct{}

	changed func()
	store   *stateStore

	downstreamErrLog logLimiter
	sourceErrLog     logLimiter
	tooLargeLog      logLimiter
	removalLog       [numRemoveReasons]logLimiter
}

var _ connector.Logs = (*replayConnector)(nil)

func newConnector(set connector.Settings, cfg *Config, next consumer.Logs) (*replayConnector, error) {
	rules, err := compileRules(cfg, set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	specs := make([]ruleSpec, len(cfg.Rules))
	for i := range cfg.Rules {
		rc := &cfg.Rules[i]
		specs[i] = ruleSpec{name: rc.Name, identity: rc.Identity, maxEntries: rc.effectiveMaxEntries()}
	}
	return &replayConnector{
		id:           set.ID,
		logger:       set.Logger,
		cfg:          cfg,
		next:         next,
		rules:        rules,
		state:        newState(specs, cfg.MaxStateBytes),
		now:          time.Now,
		checkSource:  lstatFile,
		recheckDelay: sourceRecheckDelay,
		startupDelay: startupReplayDelay,
		jitter:       randomJitter,
		changed:      func() {},
	}, nil
}

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

// Shutdown waits a bounded time for a running tick, so that the final save holds the time of the tick.
func (c *replayConnector) Shutdown(ctx context.Context) error {
	c.cancelReplaying()
	waitCtx, cancel := replayWaitContext(ctx)
	_ = c.waitReplaying(waitCtx)
	cancel()
	var err error
	if c.store != nil {
		err = c.store.shutdown(ctx)
	}
	return errors.Join(err, c.waitReplaying(ctx))
}

const maxReplayWait = 5 * time.Second

// replayWaitContext limits ctx to half of its remaining time, or to maxReplayWait without a deadline.
func replayWaitContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok {
		return context.WithTimeout(ctx, time.Until(deadline)/2)
	}
	return context.WithTimeout(ctx, maxReplayWait)
}

// Capabilities: MutatesData false lets the fanout of the shared logs pipeline
// pass the same data to every exporter without a copy.
func (c *replayConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// batch is allocated only after a record passes body_contains.
type batch struct {
	idents []identityInfo
	obs    []observation
}

type identityInfo struct {
	resolved bool
	ok       bool
	key      string
	identity []kv
}

// ConsumeLogs sends nothing, because the logs pipeline already exports the original record.
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
					if !c.rules[ri].matchesBody(lr) {
						continue
					}
					if b == nil {
						b = &batch{idents: make([]identityInfo, len(c.rules))}
					}
					if err = c.observe(ctx, b, ri, rl, sl, lr); err != nil {
						break scan
					}
				}
			}
		}
	}
	c.apply(b)
	return err
}

func (c *replayConnector) observe(ctx context.Context, b *batch, ri int, rl plog.ResourceLogs, sl plog.ScopeLogs, lr plog.LogRecord) error {
	rule := &c.rules[ri]
	if rule.conditions != nil {
		tCtx := ottllog.NewTransformContextPtr(rl, sl, lr)
		match, err := rule.matchesConditions(ctx, tCtx)
		tCtx.Close()
		if err != nil {
			return fmt.Errorf("rule %q: %w", rule.cfg.Name, err)
		}
		if !match {
			return nil
		}
	}

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
		return nil
	}

	src, ok := sourceFile(lr, c.cfg.SourceFileAttribute)
	if !ok {
		if ce := c.logger.Check(zap.DebugLevel, "Matching log record not kept: the source file attribute is missing or empty"); ce != nil {
			ce.Write(zap.String("rule", rule.cfg.Name), zap.String("source_file_attribute", c.cfg.SourceFileAttribute))
		}
		return nil
	}

	data, err := encodeRecord(rl, sl, lr)
	if err != nil {
		if ce := c.logger.Check(zap.DebugLevel, "Matching log record not kept: it cannot be encoded"); ce != nil {
			ce.Write(zap.String("rule", rule.cfg.Name), zap.Error(err))
		}
		return nil
	}
	if len(data) > c.cfg.MaxRecordBytes {
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

func (c *replayConnector) apply(b *batch) {
	if b == nil || len(b.obs) == 0 {
		return
	}
	res := c.state.applyBatch(b.obs)
	if res.changed() {
		c.changed()
	}
	c.logRemovals(res.removed, res.rejected)
}

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

// A time ahead of now is now, so that a skewed clock cannot block eviction.
func (c *replayConnector) recordTime(lr plog.LogRecord) int64 {
	now := c.now().UnixNano()
	if ts := recordTimestamp(lr); ts != 0 {
		return min(ts.AsTime().UnixNano(), now)
	}
	return now
}

const logInterval = time.Minute

// logLimiter allows one log per logInterval and counts the suppressed ones.
type logLimiter struct {
	last       atomic.Int64
	suppressed atomic.Int64
}

// allow reports whether a log is allowed at now (Unix nanoseconds), with the
// count suppressed since the previous one, or adds n to that count.
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
