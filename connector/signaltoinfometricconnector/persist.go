// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"unicode/utf8"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/statestore"
)

// The state is one record in the storage extension. Memory is the only
// source of the emission: the connector reads the record once, in Start, and
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
// its entities, each with its separating comma. The record adds the
// envelope, {"version":1,"data":{"rules":{}}}, and for each rule with
// entities "<rule name>":{} and a comma.
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
// hooks stay no-ops, and there is no store telemetry, as without storage.
// Persistence is optional, so a node with an unusable storage directory
// keeps its telemetry.
//
// A failure to read the record is not an error either. The connector logs
// it and starts with an empty state.
func (c *infoMetricConnector) startPersistence(ctx context.Context, host component.Host) error {
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
	if err := c.tel.registerStore(store.Stats); err != nil {
		// The store owns the client, and Shutdown without Start closes it.
		return errors.Join(fmt.Errorf("failed to create the state store telemetry: %w", err), store.Shutdown(ctx))
	}
	c.store = store
	rewrite := c.restoreState(ctx)
	// The collector starts a connector before the receivers of its logs
	// pipeline, so ConsumeLogs does not run before Start returns, and the
	// hooks are in place for its first batch.
	c.hooks = stateHooks{changed: store.MarkChanged, touched: store.MarkTouched}
	store.Start(func() (stateSnapshot, error) { return c.state.snapshot(), nil })
	if rewrite {
		store.MarkChanged()
	}
	return nil
}

// logStorageUnavailable logs that the connector runs without persistence.
// Start calls it at most once.
func (c *infoMetricConnector) logStorageUnavailable(err error) {
	c.logger.Error("Failed to open the Info metric state storage. The state is kept in memory only and does not survive a restart.",
		zap.Error(err),
		zap.String("storage", c.cfg.Storage.String()),
	)
}

// restoreState loads the stored record, fits it to the current
// configuration, and replaces the state with it. It reports whether the
// record needs a rewrite with the current state, which is the case when
// there is no record, when the record is not usable, and when the restore
// dropped or changed entities. The rewrite comes within Debounce, so a
// corrupt record or one of another version is logged once, not at every
// start, and the record matches the configuration even if nothing changes
// before the next restart.
//
// statestore logs a record that is corrupt or has another version, and
// Load returns no record. A storage read error is logged here, and is not a
// reason for a rewrite: the record can be valid and the error transient. In
// these cases the state stays empty, and the facts come back as the pods
// log them again.
func (c *infoMetricConnector) restoreState(ctx context.Context) bool {
	snap, found, err := c.store.Load(ctx)
	if err != nil {
		c.logger.Warn("Failed to read the stored Info metric state. Starting with an empty state.", zap.Error(err))
		return false
	}
	if !found {
		return true
	}
	norm := normalizeSnapshot(&snap, c.cfg)
	kept, dropped := c.state.restore(snap)
	dropped += norm.dropped
	c.logger.Info("Restored the Info metric state",
		zap.Int("kept", kept),
		zap.Int("dropped", dropped),
		zap.Int("normalized", norm.changed),
	)
	return dropped > 0 || norm.changed > 0
}

// normalizeResult counts what normalizeSnapshot did.
type normalizeResult struct {
	// dropped counts the entities removed.
	dropped int
	// changed counts the entities whose facts or resource changed.
	changed int
}

// normalizeSnapshot fits each entity of a configured rule to the current
// configuration of the rule, in place, so that the restored entity is the
// one that the same record gives under this configuration:
//
//   - Facts: only the configured attributes, in config order, with the
//     values made valid UTF-8 and cut to max_value_length by fitValue. An
//     entity without a value for a required attribute (no default_value,
//     not optional) is dropped, because a record without that value is not
//     used. An attribute without a stored value is otherwise absent:
//     default_value replaces the result of the value expression, and the
//     stored entity has no record to evaluate.
//   - Resource: only the include_resource_attributes, in that order, with
//     the values fitted the same way.
//   - Identity: an entity with an identity value longer than
//     max_value_length, or not valid UTF-8, is dropped, because no record
//     gives its state key any more.
//
// The JSON decoder already replaces each invalid UTF-8 byte of the record
// with U+FFFD. The checks here do not depend on it, so a restored entity
// cannot bring invalid UTF-8 back into the emission.
//
// state.restore drops the entities of rules that are no longer configured
// and the entities whose identity does not match the rule.
func normalizeSnapshot(snap *stateSnapshot, cfg *Config) normalizeResult {
	var res normalizeResult
	for i := range cfg.Rules {
		rc := &cfg.Rules[i]
		entities := snap.Rules[rc.Name]
		for key, e := range entities {
			next, ok := normalizeEntity(e, rc, cfg.MaxValueLength)
			switch {
			case !ok:
				delete(entities, key)
				res.dropped++
			case !slices.Equal(next.Facts, e.Facts) || !slices.Equal(next.Resource, e.Resource):
				entities[key] = next
				res.changed++
			}
		}
	}
	return res
}

// normalizeEntity returns e fitted to the rule, or false when the entity is
// to be dropped; see normalizeSnapshot.
func normalizeEntity(e entity, rc *RuleConfig, maxLen int) (entity, bool) {
	for _, p := range e.Identity {
		if len(p.Value) > maxLen || !utf8.ValidString(p.Value) {
			return entity{}, false
		}
	}
	facts := make([]kv, 0, len(rc.Attributes))
	for i := range rc.Attributes {
		attr := &rc.Attributes[i]
		if v := fitValue(pairValue(e.Facts, attr.Key), maxLen); v != "" {
			facts = append(facts, kv{Key: attr.Key, Value: v})
		} else if attr.DefaultValue == "" && !attr.Optional {
			return entity{}, false
		}
	}
	var resource []kv
	for _, k := range rc.IncludeResourceAttributes {
		if v := fitValue(pairValue(e.Resource, k), maxLen); v != "" {
			resource = append(resource, kv{Key: k, Value: v})
		}
	}
	e.Facts, e.Resource = facts, resource
	return e, true
}

// pairValue returns the value of the first pair with the key, or "".
func pairValue(pairs []kv, key string) string {
	for _, p := range pairs {
		if p.Key == key {
			return p.Value
		}
	}
	return ""
}
