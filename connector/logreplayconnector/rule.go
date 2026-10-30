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

// compiledRule is a RuleConfig with its OTTL compiled for the hot path.
type compiledRule struct {
	cfg *RuleConfig
	// conditions is nil when the rule has no conditions.
	conditions *ottl.ConditionSequence[*ottllog.TransformContext]
}

// compileRules compiles the OTTL of every rule. Validate has already parsed
// the same conditions, so an error here means the configuration was not
// validated.
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

// matchesBody applies the body_contains check: the body must be a string
// that contains at least one of the substrings. A rule without
// body_contains passes. It does not allocate, because it runs for every
// record and rule.
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

// matchesConditions applies the OTTL conditions, ORed. A rule without
// conditions passes. The error is non-nil only in propagate mode; the
// condition sequence logs (ignore) or drops (silent) errors otherwise.
func (r *compiledRule) matchesConditions(ctx context.Context, tCtx *ottllog.TransformContext) (bool, error) {
	if r.conditions == nil {
		return true, nil
	}
	return r.conditions.Eval(ctx, tCtx)
}

// matches applies both checks to a record of a ResourceLogs and ScopeLogs.
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

// resolveIdentity reads the identity of a resource. The values are
// converted with AsString, made valid UTF-8, and cloned, so that the state
// does not keep the input's memory alive. It returns false when an identity
// attribute is missing or empty.
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

// encodeIdentity encodes identity values as a state key. Each value is
// prefixed with its length ("<len>:<value>"), so different value lists
// never produce the same key.
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

// validUTF8 returns s with each run of invalid UTF-8 bytes replaced by
// U+FFFD. A valid s is checked without allocating and returned as is.
func validUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "\uFFFD")
}

// sourceFile returns the value of the source file attribute key of a
// record, and false when the attribute is absent or empty.
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

// encodeRecord returns one record with its resource and scope as OTLP JSON:
// one ResourceLogs with one ScopeLogs with one LogRecord. It copies the
// record, so the input is not changed.
//
// The JSON is compact, and '<', '>', '&', U+2028, and U+2029 are escaped
// as encoding/json escapes them. The state record embeds it as a
// json.RawMessage, which encoding/json compacts and escapes the same way,
// so the length of the result is its length in the state record.
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

// decodeRecord parses a record that encodeRecord wrote. It returns an error
// unless the JSON holds exactly one ResourceLogs with one ScopeLogs with one
// LogRecord.
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

// recordTimestamp returns the record's timestamp, else its observed
// timestamp, else 0.
func recordTimestamp(lr plog.LogRecord) pcommon.Timestamp {
	if ts := lr.Timestamp(); ts != 0 {
		return ts
	}
	return lr.ObservedTimestamp()
}
