// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"cmp"
	"context"
	"errors"
	"io/fs"
	"math/rand/v2"
	"os"
	"slices"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

// scopeName is the name of the meter of the connector's own telemetry. The
// copies keep the scope of the original record.
const scopeName = "github.com/aws/amazon-cloudwatch-agent/connector/logreplayconnector"

// The first tick comes startupReplayDelay plus a random jitter in
// [0, min(interval, maxStartupJitter)) after Start. After a restart, the
// first tick sends the restored records, so the delay lets the exporters of
// the next pipeline connect first, and the jitter spreads the ticks of the
// agents of a cluster that start together. If the restored state holds the
// time of the latest copies, the first tick waits until one interval after
// them instead.
const (
	startupReplayDelay = 10 * time.Second
	maxStartupJitter   = 10 * time.Second
)

// sourceRecheckDelay is the time between the two checks of a source file
// that is missing. A log rotation that renames the file and then creates a
// new one leaves it missing for a moment.
const sourceRecheckDelay = time.Second

// maxSendBytes bounds the size of the kept records in one call to the next
// consumer, so that a large state goes out in several calls. A record
// larger than this goes out alone.
const maxSendBytes = 1 << 20

// randomJitter returns a random duration in [0, n). n must be positive.
func randomJitter(n time.Duration) time.Duration {
	return rand.N(n) //nolint:gosec // G404: a start jitter does not need a secure random source.
}

// lstatFile returns nil when path exists. A symbolic link counts as
// existing, also when its target is missing. The kubelet removes the link in
// /var/log/containers when it removes the container. While it rotates the
// container log, the target of the link is missing for a moment.
func lstatFile(path string) error {
	_, err := os.Lstat(path)
	return err
}

// startReplaying launches the replay goroutine and returns at once.
func (c *replayConnector) startReplaying() {
	ctx, cancel := context.WithCancel(context.Background())
	delay := c.firstDelay()
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.run(ctx, delay)
	}()
	c.stopReplay, c.replayDone = cancel, done
}

// firstDelay returns the time from Start to the first tick: the startup
// delay plus the jitter, or, when the restored state says when the latest
// copies went out, one interval after them if that is later. The delay is
// at most one interval, also when the stored time is ahead of the clock.
func (c *replayConnector) firstDelay() time.Duration {
	delay := c.startupDelay + c.jitter(min(c.cfg.Interval, maxStartupJitter))
	if last := c.state.lastReplayTime(); last > 0 {
		due := time.Duration(last + c.cfg.Interval.Nanoseconds() - c.now().UnixNano())
		delay = max(delay, min(due, c.cfg.Interval))
	}
	return delay
}

// cancelReplaying cancels the context of the replay and returns at once. It
// is safe to call before startReplaying and more than once.
func (c *replayConnector) cancelReplaying() {
	if c.stopReplay != nil {
		c.stopReplay()
	}
}

// waitReplaying waits for the replay to stop after cancelReplaying, bounded
// by ctx. It is safe to call before startReplaying and more than once.
func (c *replayConnector) waitReplaying(ctx context.Context) error {
	if c.replayDone == nil {
		return nil
	}
	select {
	case <-c.replayDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run ticks after delay, then every interval, until ctx is done.
func (c *replayConnector) run(ctx context.Context, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	ticker := time.NewTicker(c.cfg.Interval)
	defer ticker.Stop()
	for {
		c.tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// chunk is one call to the next consumer, with the number of copies of
// each rule in it.
type chunk struct {
	ld   plog.Logs
	sent []int64
}

// tick removes the kept records whose source file is gone, and sends a copy
// of every other kept record, in calls of at most maxSendBytes of records.
// The entries are copied under the state lock, and the files are checked
// and the copies built and sent without it. An empty state sends nothing.
// When every call succeeds, the tick time is stored as the time of the
// latest copies.
func (c *replayConnector) tick(ctx context.Context) {
	kept := c.state.entries()
	if len(kept) == 0 {
		return
	}
	live, gone, skipped := c.checkSources(ctx, kept)
	if ctx.Err() != nil {
		return
	}
	if removed := c.state.removeGone(gone); len(removed) > 0 {
		c.changed()
		c.tel.recordRemovals(ctx, removed, nil)
		c.logRemovals(removed, nil)
	}

	ts := pcommon.NewTimestampFromTime(c.now())
	var chunks []chunk
	size := 0
	for i := range live {
		k := &live[i]
		ld, err := decodeRecord(k.entry.Record)
		if err != nil {
			// Not expected: the connector encoded the record, and the
			// restore checks every stored record.
			skipped = append(skipped, skip{rule: k.rule, reason: skipDecodeError})
			if ce := c.logger.Check(zap.DebugLevel, "Kept log record not sent: it cannot be decoded"); ce != nil {
				ce.Write(zap.String("rule", c.rules[k.rule].cfg.Name), zap.Error(err))
			}
			continue
		}
		if len(chunks) == 0 || (size > 0 && size+len(k.entry.Record) > maxSendBytes) {
			chunks = append(chunks, chunk{ld: plog.NewLogs(), sent: make([]int64, len(c.rules))})
			size = 0
		}
		cur := &chunks[len(chunks)-1]
		setTimes(ld, ts)
		ld.ResourceLogs().MoveAndAppendTo(cur.ld.ResourceLogs())
		cur.sent[k.rule]++
		size += len(k.entry.Record)
	}
	c.tel.recordSkipped(ctx, skipped)
	if len(chunks) == 0 {
		return
	}

	ok := true
	for i := range chunks {
		err := c.next.ConsumeLogs(ctx, chunks[i].ld)
		if err != nil && ctx.Err() != nil {
			if ce := c.logger.Check(zap.DebugLevel, "Log record copies not sent: the replay was canceled"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}
		c.tel.recordReplay(ctx, chunks[i].sent, err)
		if err != nil {
			ok = false
			c.logDownstreamError(err)
		}
	}
	if ok {
		c.state.setLastReplay(ts.AsTime().UnixNano())
		c.changed()
	}
}

// checkSources sorts the kept records by their source file: live (the file
// exists), gone (the file is missing at two checks, sourceRecheckDelay
// apart), and skipped (a check failed for another reason, so the record is
// not sent and stays kept). live keeps the order of kept. If ctx ends
// during the wait, no record is gone.
func (c *replayConnector) checkSources(ctx context.Context, kept []keptEntry) ([]keptEntry, []keptEntry, []skip) {
	var live, missing, gone []keptEntry
	var skipped []skip
	sortOut := func(k keptEntry, notExist *[]keptEntry) {
		switch err := c.checkSource(k.entry.SourceFile); {
		case err == nil:
			live = append(live, k)
		case errors.Is(err, fs.ErrNotExist):
			*notExist = append(*notExist, k)
		default:
			skipped = append(skipped, skip{rule: k.rule, reason: skipSourceCheckError})
			c.logSourceError(c.rules[k.rule].cfg.Name, k.entry.SourceFile, err)
		}
	}
	for _, k := range kept {
		sortOut(k, &missing)
	}
	if len(missing) == 0 {
		return live, nil, skipped
	}
	if c.recheckDelay > 0 {
		timer := time.NewTimer(c.recheckDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return live, nil, skipped
		case <-timer.C:
		}
	}
	for _, k := range missing {
		sortOut(k, &gone)
	}
	slices.SortFunc(live, func(a, b keptEntry) int {
		return cmp.Or(cmp.Compare(a.rule, b.rule), cmp.Compare(a.key, b.key))
	})
	return live, gone, skipped
}

// setTimes sets the timestamp and the observed timestamp of every record of
// ld to ts. Nothing else of a copy differs from the original.
func setTimes(ld plog.Logs, ts pcommon.Timestamp) {
	for _, rl := range ld.ResourceLogs().All() {
		for _, sl := range rl.ScopeLogs().All() {
			for _, lr := range sl.LogRecords().All() {
				lr.SetTimestamp(ts)
				lr.SetObservedTimestamp(ts)
			}
		}
	}
}

// logDownstreamError logs an error of the next consumer at most once per
// logInterval. The error does not go back to the logs pipeline that feeds
// the connector, and a tick has no caller to return it to.
func (c *replayConnector) logDownstreamError(err error) {
	suppressed, ok := c.downstreamErrLog.allow(c.now().UnixNano(), 1)
	if !ok {
		return
	}
	c.logger.Warn("Failed to send the log record copies to the next consumer",
		zap.Error(err),
		zap.Int64("suppressed_since_last_log", suppressed),
	)
}

// logSourceError logs a source file that could not be checked, at most once
// per logInterval.
func (c *replayConnector) logSourceError(rule, path string, err error) {
	const msg = "Kept log record not sent: its source file cannot be checked"
	if suppressed, ok := c.sourceErrLog.allow(c.now().UnixNano(), 1); ok {
		c.logger.Warn(msg,
			zap.String("rule", rule),
			zap.String("source_file", path),
			zap.Error(err),
			zap.Int64("suppressed_since_last_log", suppressed),
		)
		return
	}
	if ce := c.logger.Check(zap.DebugLevel, msg); ce != nil {
		ce.Write(zap.String("rule", rule), zap.String("source_file", path), zap.Error(err))
	}
}
