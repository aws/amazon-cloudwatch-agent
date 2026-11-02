// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type compiledRule struct {
	cfg        *RuleConfig
	conditions *ottl.ConditionSequence[*ottllog.TransformContext]
}

func compileRules(cfg *Config, set component.TelemetrySettings) ([]compiledRule, error) {
	parser, err := newLogParser(set)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTTL log parser: %w", err)
	}
	rules := make([]compiledRule, len(cfg.Rules))
	for i := range cfg.Rules {
		rc := &cfg.Rules[i]
		rule := compiledRule{cfg: rc}
		if len(rc.Conditions) > 0 {
			conditions, err := parser.ParseConditions(rc.Conditions)
			if err != nil {
				return nil, fmt.Errorf("%s: conditions: %w", rc.label(i), err)
			}
			seq := ottl.NewConditionSequence(conditions, set,
				ottl.WithLogicOperation[*ottllog.TransformContext](ottl.Or),
				ottl.WithConditionSequenceErrorMode[*ottllog.TransformContext](cfg.ErrorMode),
			)
			rule.conditions = &seq
		}
		rules[i] = rule
	}
	return rules, nil
}

// matchesBody runs for every record and rule, so it does not allocate.
func (r *compiledRule) matchesBody(lr plog.LogRecord) bool {
	subs := r.cfg.BodyContains
	if len(subs) == 0 {
		return true
	}
	body := lr.Body()
	if body.Type() != pcommon.ValueTypeStr {
		return false
	}
	s := body.Str()
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func (r *compiledRule) matchesConditions(ctx context.Context, tCtx *ottllog.TransformContext) (bool, error) {
	if r.conditions == nil {
		return true, nil
	}
	return r.conditions.Eval(ctx, tCtx)
}

func (r *compiledRule) matches(ctx context.Context, rl plog.ResourceLogs, sl plog.ScopeLogs, lr plog.LogRecord) (bool, error) {
	if !r.matchesBody(lr) {
		return false, nil
	}
	if r.conditions == nil {
		return true, nil
	}
	tCtx := ottllog.NewTransformContextPtr(rl, sl, lr)
	defer tCtx.Close()
	return r.matchesConditions(ctx, tCtx)
}

// resolveIdentity clones the values, so that the state does not keep the input's memory alive.
func (r *compiledRule) resolveIdentity(attrs pcommon.Map) ([]kv, string, bool) {
	identity := make([]kv, 0, len(r.cfg.Identity))
	for _, k := range r.cfg.Identity {
		v, ok := attrs.Get(k)
		if !ok {
			return nil, "", false
		}
		s := validUTF8(v.AsString())
		if s == "" {
			return nil, "", false
		}
		identity = append(identity, kv{Key: k, Value: strings.Clone(s)})
	}
	return identity, encodeIdentity(identity), true
}

// encodeIdentity prefixes each value with its length ("<len>:<value>"), so
// different value lists never give the same key.
func encodeIdentity(identity []kv) string {
	n := 0
	for _, p := range identity {
		n += len(p.Value) + 4
	}
	var b strings.Builder
	b.Grow(n)
	for _, p := range identity {
		b.WriteString(strconv.Itoa(len(p.Value)))
		b.WriteByte(':')
		b.WriteString(p.Value)
	}
	return b.String()
}

func validUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "\uFFFD")
}

func sourceFile(lr plog.LogRecord, key string) (string, bool) {
	v, ok := lr.Attributes().Get(key)
	if !ok {
		return "", false
	}
	s := v.AsString()
	if s == "" {
		return "", false
	}
	return strings.Clone(s), true
}

var (
	jsonMarshaler   plog.JSONMarshaler
	jsonUnmarshaler plog.JSONUnmarshaler
)

// encodeRecord escapes as encoding/json does, so that the length of the result is its stored length.
func encodeRecord(rl plog.ResourceLogs, sl plog.ScopeLogs, lr plog.LogRecord) ([]byte, error) {
	ld := plog.NewLogs()
	orl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().CopyTo(orl.Resource())
	orl.SetSchemaUrl(rl.SchemaUrl())
	osl := orl.ScopeLogs().AppendEmpty()
	sl.Scope().CopyTo(osl.Scope())
	osl.SetSchemaUrl(sl.SchemaUrl())
	lr.CopyTo(osl.LogRecords().AppendEmpty())
	raw, err := jsonMarshaler.MarshalLogs(ld)
	if err != nil {
		return nil, err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return nil, err
	}
	var escaped bytes.Buffer
	escaped.Grow(compact.Len())
	json.HTMLEscape(&escaped, compact.Bytes())
	return escaped.Bytes(), nil
}

func decodeRecord(data []byte) (plog.Logs, error) {
	ld, err := jsonUnmarshaler.UnmarshalLogs(data)
	if err != nil {
		return plog.Logs{}, err
	}
	rls := ld.ResourceLogs()
	if rls.Len() != 1 || rls.At(0).ScopeLogs().Len() != 1 || rls.At(0).ScopeLogs().At(0).LogRecords().Len() != 1 {
		return plog.Logs{}, fmt.Errorf("the record must hold exactly one log record, got %d", ld.LogRecordCount())
	}
	return ld, nil
}

func recordTimestamp(lr plog.LogRecord) pcommon.Timestamp {
	if ts := lr.Timestamp(); ts != 0 {
		return ts
	}
	return lr.ObservedTimestamp()
}
