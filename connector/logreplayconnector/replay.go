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

// The startup delay lets the next exporters connect first. The jitter spreads the agents of a cluster.
const (
	startupReplayDelay = 10 * time.Second
	maxStartupJitter   = 10 * time.Second
)

// A log rotation leaves the source file missing for a moment.
const sourceRecheckDelay = time.Second

const maxSendBytes = 1 << 20

func randomJitter(n time.Duration) time.Duration {
	return rand.N(n) //nolint:gosec // G404: a start jitter does not need a secure random source.
}

// lstatFile checks the link itself. The kubelet removes the link with the container, but a log
// rotation leaves only its target missing for a moment.
func lstatFile(path string) error {
	_, err := os.Lstat(path)
	return err
}

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

func (c *replayConnector) firstDelay() time.Duration {
	delay := c.startupDelay + c.jitter(min(c.cfg.Interval, maxStartupJitter))
	if last := c.state.lastReplayTime(); last > 0 {
		due := time.Duration(last + c.cfg.Interval.Nanoseconds() - c.now().UnixNano())
		delay = max(delay, min(due, c.cfg.Interval))
	}
	return delay
}

// cancelReplaying and waitReplaying are safe before startReplaying and more
// than once.
func (c *replayConnector) cancelReplaying() {
	if c.stopReplay != nil {
		c.stopReplay()
	}
}

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

// tick holds no state lock while it checks files and sends. It stores the tick time only if every
// call succeeds.
func (c *replayConnector) tick(ctx context.Context) {
	kept := c.state.entries()
	if len(kept) == 0 {
		return
	}
	// The time is taken before the source checks, so that a recheck does not
	// stretch the gap between two copies beyond the interval.
	ts := pcommon.NewTimestampFromTime(c.now())
	live, gone := c.checkSources(ctx, kept)
	if ctx.Err() != nil {
		return
	}
	if removed := c.state.removeGone(gone); len(removed) > 0 {
		c.changed()
		c.logRemovals(removed, nil)
	}

	var chunks []plog.Logs
	size := 0
	for i := range live {
		k := &live[i]
		ld, err := decodeRecord(k.entry.Record)
		if err != nil {
			// Not expected: the restore checks every stored record.
			if ce := c.logger.Check(zap.DebugLevel, "Kept log record not sent: it cannot be decoded"); ce != nil {
				ce.Write(zap.String("rule", c.rules[k.rule].cfg.Name), zap.Error(err))
			}
			continue
		}
		if len(chunks) == 0 || (size > 0 && size+len(k.entry.Record) > maxSendBytes) {
			chunks = append(chunks, plog.NewLogs())
			size = 0
		}
		setTimes(ld, ts)
		ld.ResourceLogs().MoveAndAppendTo(chunks[len(chunks)-1].ResourceLogs())
		size += len(k.entry.Record)
	}
	if len(chunks) == 0 {
		return
	}

	ok := true
	for i := range chunks {
		err := c.next.ConsumeLogs(ctx, chunks[i])
		if err != nil && ctx.Err() != nil {
			if ce := c.logger.Check(zap.DebugLevel, "Log record copies not sent: the replay was canceled"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}
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

// checkSources returns the live records and the gone records (missing at both checks). A record
// whose check fails is in neither, so it stays kept but is not sent.
func (c *replayConnector) checkSources(ctx context.Context, kept []keptEntry) ([]keptEntry, []keptEntry) {
	var live, missing, gone []keptEntry
	sortOut := func(k keptEntry, notExist *[]keptEntry) {
		switch err := c.checkSource(k.entry.SourceFile); {
		case err == nil:
			live = append(live, k)
		case errors.Is(err, fs.ErrNotExist):
			*notExist = append(*notExist, k)
		default:
			c.logSourceError(c.rules[k.rule].cfg.Name, k.entry.SourceFile, err)
		}
	}
	for _, k := range kept {
		sortOut(k, &missing)
	}
	if len(missing) == 0 {
		return live, nil
	}
	if c.recheckDelay > 0 {
		timer := time.NewTimer(c.recheckDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return live, nil
		case <-timer.C:
		}
	}
	for _, k := range missing {
		sortOut(k, &gone)
	}
	slices.SortFunc(live, func(a, b keptEntry) int {
		return cmp.Or(cmp.Compare(a.rule, b.rule), cmp.Compare(a.key, b.key))
	})
	return live, gone
}

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
