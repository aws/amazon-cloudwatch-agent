// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// startPersistence fails only for a wrong storage configuration. Other storage errors keep the
// state in memory only, and leave the stored record as it is.
func (c *replayConnector) startPersistence(ctx context.Context, host component.Host) error {
	client, err := getStorageClient(ctx, host, *c.cfg.Storage, c.id)
	if errors.Is(err, errExtensionNotFound) || errors.Is(err, errNotStorageExtension) {
		return fmt.Errorf("failed to get the storage client: %w", err)
	}
	if err != nil {
		c.logStorageUnavailable(err)
		return nil
	}
	store := newStateStore(client, c.logger)
	snap, found, err := store.load(ctx)
	if err != nil {
		c.logger.Warn("Failed to read the stored log_replay state. The kept log records are in memory only until the next restart, so that the stored state is not replaced.",
			zap.Error(err))
		if err := store.shutdown(ctx); err != nil {
			c.logger.Debug("Failed to close the log_replay state storage", zap.Error(err))
		}
		return nil
	}
	c.store = store
	rewrite := c.restoreState(ctx, snap, found)
	// The receivers of the logs pipeline start after the connector.
	c.changed = store.markChanged
	store.start(c.state.snapshot)
	if rewrite {
		store.markChanged()
	}
	return nil
}

func (c *replayConnector) logStorageUnavailable(err error) {
	c.logger.Error("Failed to open the log_replay state storage. The kept log records are in memory only and do not survive a restart.",
		zap.Error(err),
		zap.String("storage", c.cfg.Storage.String()),
	)
}

// restoreState reports whether the record needs a rewrite, so that a bad record is logged once.
func (c *replayConnector) restoreState(ctx context.Context, snap stateSnapshot, found bool) bool {
	if !found {
		return true
	}
	items, res := c.checkSnapshot(ctx, &snap)
	now := c.now().UnixNano()
	kept, dropped := c.state.restore(items, max(0, min(snap.LastReplay, now)))
	dropped += res.dropped
	c.logger.Info("Restored the log_replay state",
		zap.Int("kept", kept),
		zap.Int("dropped", dropped),
		zap.Int("changed", res.changed),
	)
	return dropped > 0 || res.changed > 0
}

type checkResult struct {
	dropped int
	changed int
}

func (c *replayConnector) checkSnapshot(ctx context.Context, snap *stateSnapshot) ([]restoredEntry, checkResult) {
	var res checkResult
	var items []restoredEntry
	now := c.now().UnixNano()
	for name, entries := range snap.Rules {
		ri, ok := c.state.ruleIndex[name]
		if !ok {
			res.dropped += len(entries)
			continue
		}
		for key, e := range entries {
			src, ok := c.checkEntry(ctx, ri, key, &e)
			if !ok {
				res.dropped++
				continue
			}
			if src != e.SourceFile || e.Time > now {
				e.SourceFile = src
				e.Time = min(e.Time, now)
				res.changed++
			}
			items = append(items, restoredEntry{entryRef: entryRef{rule: ri, key: key}, entry: e})
		}
	}
	return items, res
}

func (c *replayConnector) checkEntry(ctx context.Context, ri int, key string, e *entry) (string, bool) {
	if len(e.Record) > c.cfg.MaxRecordBytes || e.Time <= 0 {
		return "", false
	}
	ld, err := decodeRecord(e.Record)
	if err != nil {
		return "", false
	}
	rl := ld.ResourceLogs().At(0)
	sl := rl.ScopeLogs().At(0)
	lr := sl.LogRecords().At(0)
	rule := &c.rules[ri]
	if match, err := rule.matches(ctx, rl, sl, lr); err != nil || !match {
		return "", false
	}
	identity, k, ok := rule.resolveIdentity(rl.Resource().Attributes())
	if !ok || k != key || !slices.Equal(identity, e.Identity) {
		return "", false
	}
	return sourceFile(lr, c.cfg.SourceFileAttribute)
}
