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

	"github.com/aws/amazon-cloudwatch-agent/internal/statestore"
)

// The state is one record in the storage extension. Memory is the only
// source of the copies: the connector reads the record once, in Start, and
// after that only writes it.
const (
	stateRecordKey = "state"
	// stateRecordVersion is the version of the JSON form of stateSnapshot.
	// statestore discards a record with another version, so a change of the
	// form needs a new version.
	stateRecordVersion = 1
)

// Margins of the record size limit over max_state_bytes. The size estimate
// of the state, which max_state_bytes bounds, is at least the JSON size of
// its entries, each with its separating comma. The record adds the
// envelope, {"version":1,"data":{"rules":{}}}, and for each rule with
// entries "<rule name>":{} and a comma.
const (
	recordEnvelopeMargin = 4 << 10
	recordRuleMargin     = 16
)

// maxRecordBytes returns the size limit of the stored record. A state within
// max_state_bytes always fits, so the store never refuses it as too large.
func maxRecordBytes(cfg *Config) int {
	n := cfg.MaxStateBytes + recordEnvelopeMargin
	for i := range cfg.Rules {
		n += jsonStringLen(cfg.Rules[i].Name) + recordRuleMargin
	}
	return n
}

// startPersistence opens the state store, restores the stored state, and
// starts the saver. It returns an error when the storage extension is
// missing or is not a storage extension, because then the configuration is
// wrong.
//
// If the extension cannot give a client (for example, file_storage cannot
// open or create its file), or the store cannot be created, the connector
// logs one Error and keeps the state in memory only. c.store stays nil, the
// hook stays a no-op, and there is no store telemetry, as without storage.
//
// A failure to read the record is not an error either. The connector logs
// it and keeps the records in memory only for this run, so that its first
// save does not replace a stored record that may still be valid.
func (c *replayConnector) startPersistence(ctx context.Context, host component.Host) error {
	client, err := statestore.GetClient(ctx, host, c.cfg.Storage, component.KindConnector, c.id)
	if errors.Is(err, statestore.ErrExtensionNotFound) || errors.Is(err, statestore.ErrNotStorageExtension) {
		return fmt.Errorf("failed to get the storage client: %w", err)
	}
	if err != nil {
		c.logStorageUnavailable(err)
		return nil
	}
	store, err := statestore.New[stateSnapshot](client, statestore.Options{
		Key:      stateRecordKey,
		Version:  stateRecordVersion,
		MaxBytes: maxRecordBytes(c.cfg),
		Logger:   c.logger,
	})
	if err != nil {
		err = fmt.Errorf("failed to create the state store: %w", err)
		// An extension can return a nil client without an error, and New
		// refuses it.
		if client != nil {
			err = errors.Join(err, client.Close(ctx))
		}
		c.logStorageUnavailable(err)
		return nil
	}
	snap, found, err := store.Load(ctx)
	if err != nil {
		c.logger.Warn("Failed to read the stored log_replay state. The kept log records are in memory only until the next restart, so that the stored state is not replaced.",
			zap.Error(err))
		// The store owns the client, and Shutdown without Start closes it.
		if err := store.Shutdown(ctx); err != nil {
			c.logger.Debug("Failed to close the log_replay state storage", zap.Error(err))
		}
		return nil
	}
	if err := c.tel.registerStore(store.Stats); err != nil {
		return errors.Join(fmt.Errorf("failed to create the state store telemetry: %w", err), store.Shutdown(ctx))
	}
	c.store = store
	rewrite := c.restoreState(ctx, snap, found)
	// The collector starts a connector before the receivers of its logs
	// pipeline, so ConsumeLogs does not run before Start returns, and the
	// hook is in place for its first batch.
	c.changed = store.MarkChanged
	store.Start(func() (stateSnapshot, error) { return c.state.snapshot(), nil })
	if rewrite {
		store.MarkChanged()
	}
	return nil
}

// logStorageUnavailable logs that the connector runs without persistence.
// Start calls it at most once.
func (c *replayConnector) logStorageUnavailable(err error) {
	c.logger.Error("Failed to open the log_replay state storage. The kept log records are in memory only and do not survive a restart.",
		zap.Error(err),
		zap.String("storage", c.cfg.Storage.String()),
	)
}

// restoreState keeps the entries of a loaded record that the current
// configuration would keep, and replaces the state with them. It reports
// whether the record needs a rewrite with the current state, which is the
// case when there is no record, when the record is not usable, and when the
// restore dropped or changed entries. The rewrite comes within Debounce, so
// a corrupt record or one of another version is logged once, not at every
// start. statestore logs a record that is corrupt or has another version,
// and Load returns no record.
func (c *replayConnector) restoreState(ctx context.Context, snap stateSnapshot, found bool) bool {
	if !found {
		return true
	}
	items, res := c.checkSnapshot(ctx, &snap)
	// A time ahead of the clock (a clock step back) is now, so that it
	// cannot hold back newer records or the next copies.
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

// checkResult counts what checkSnapshot did.
type checkResult struct {
	// dropped counts the entries that the current configuration would not
	// keep.
	dropped int
	// changed counts the entries whose source file changed with the
	// configuration, or whose time was ahead of the clock.
	changed int
}

// checkSnapshot returns the entries of the snapshot that the current
// configuration would keep, each as the record would give it now: the
// record decodes to one record within max_record_bytes, its rule still
// exists and matches it, its identity resolves to the stored key, and its
// source file is read again with the current source_file_attribute. An entry
// whose record no longer has the source file attribute is dropped. A rule
// condition that fails to evaluate drops the entry. A time ahead of the
// clock becomes the current time.
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

// checkEntry reports whether the current configuration keeps a stored
// entry of rule ri under key, and returns its source file.
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
