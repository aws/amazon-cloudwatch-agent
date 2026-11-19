// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"context"
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
	attrs      []compiledAttr
}

// compiledAttr is an AttributeConfig with its value expression compiled.
type compiledAttr struct {
	key          string
	expr         *ottl.ValueExpression[*ottllog.TransformContext]
	defaultValue string
	optional     bool
}

// compileRules compiles the OTTL of every rule. Validate has already parsed
// the same expressions, so an error here means the configuration was not
// validated.
func compileRules(cfg *Config, set component.TelemetrySettings) ([]compiledRule, error) {
	parser, err := newLogParser(set)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTTL log parser: %w", err)
	}
	rules := make([]compiledRule, len(cfg.Rules))
	for i := range cfg.Rules {
		rc := &cfg.Rules[i]
		rule := compiledRule{cfg: rc, attrs: make([]compiledAttr, len(rc.Attributes))}
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
		for j := range rc.Attributes {
			ac := &rc.Attributes[j]
			expr, err := parser.ParseValueExpression(ac.valueExpression())
			if err != nil {
				return nil, fmt.Errorf("%s: attributes[%d]: %w", rc.label(i), j, err)
			}
			rule.attrs[j] = compiledAttr{key: ac.Key, expr: expr, defaultValue: ac.DefaultValue, optional: ac.Optional}
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

// resourceInfo is what a rule takes from the resource of a ResourceLogs.
// It is computed at most once per ResourceLogs and rule.
type resourceInfo struct {
	resolved bool
	// ok is false when an identity key is missing or empty.
	ok bool
	// key is the encoded identity, the key of the entity in the state.
	key      string
	identity []kv
	resource []kv
	// truncated counts the identity and resource values cut to the
	// maximum length.
	truncated int
}

// resolveResource reads the identity and the included resource attributes.
// Values are converted with AsString, made valid UTF-8 and cut to maxLen
// bytes by fitValue, and cloned so that the state does not keep the input's
// memory alive. An included attribute that is absent or empty is omitted.
// The identity key is computed from the fitted values, so the state and the
// emitted labels agree.
func (r *compiledRule) resolveResource(attrs pcommon.Map, maxLen int) resourceInfo {
	truncated := 0
	identity := make([]kv, 0, len(r.cfg.Identity))
	for _, k := range r.cfg.Identity {
		v, ok := attrs.Get(k)
		if !ok {
			return resourceInfo{resolved: true, truncated: truncated}
		}
		s := fitCounted(v.AsString(), maxLen, &truncated)
		if s == "" {
			return resourceInfo{resolved: true, truncated: truncated}
		}
		identity = append(identity, kv{Key: k, Value: strings.Clone(s)})
	}
	var resource []kv
	if len(r.cfg.IncludeResourceAttributes) > 0 {
		resource = make([]kv, 0, len(r.cfg.IncludeResourceAttributes))
	}
	for _, k := range r.cfg.IncludeResourceAttributes {
		v, ok := attrs.Get(k)
		if !ok {
			continue
		}
		if s := fitCounted(v.AsString(), maxLen, &truncated); s != "" {
			resource = append(resource, kv{Key: k, Value: strings.Clone(s)})
		}
	}
	return resourceInfo{
		resolved:  true,
		ok:        true,
		key:       encodeIdentity(identity),
		identity:  identity,
		resource:  resource,
		truncated: truncated,
	}
}

// appendIdentityKey appends the state key of the resource's identity to
// buf. The key is the one that resolveResource computes, with the same
// fitValue, so an entity whose identity had invalid UTF-8 is found. It is
// built without allocating when the identity values are strings of valid
// UTF-8. It returns false when an identity attribute is missing or empty,
// and then buf can hold part of a key past its original length.
func (r *compiledRule) appendIdentityKey(buf []byte, attrs pcommon.Map, maxLen int) ([]byte, bool) {
	for _, k := range r.cfg.Identity {
		v, ok := attrs.Get(k)
		if !ok {
			return buf, false
		}
		s := fitValue(v.AsString(), maxLen)
		if s == "" {
			return buf, false
		}
		buf = appendKeyPart(buf, s)
	}
	return buf, true
}

// factsResult is the result of extractFacts.
type factsResult struct {
	values []kv
	// missing is the key of the first required attribute without a value.
	// It is empty when the record yields facts.
	missing string
	// missingErr is the evaluation error of that attribute, if any.
	missingErr error
	// truncated counts the values cut to the maximum length.
	truncated int
}

// extractFacts evaluates the attributes in config order. Each result is
// made valid UTF-8 and cut to maxLen bytes by fitValue. A value that is an
// error, nil, or empty (also after the cut) is missing: default_value
// replaces it, optional omits it, and otherwise the record is not used
// (missing names the attribute). The error is non-nil only in
// propagate mode, when a required fact without a default fails to
// evaluate.
func (r *compiledRule) extractFacts(ctx context.Context, tCtx *ottllog.TransformContext, maxLen int, errorMode ottl.ErrorMode) (factsResult, error) {
	res := factsResult{values: make([]kv, 0, len(r.attrs))}
	for i := range r.attrs {
		attr := &r.attrs[i]
		raw, err := attr.expr.Eval(ctx, tCtx)
		var v string
		if err == nil {
			v = fitCounted(valueString(raw), maxLen, &res.truncated)
		}
		switch {
		case v != "":
			res.values = append(res.values, kv{Key: attr.key, Value: strings.Clone(v)})
		case attr.defaultValue != "":
			// Validate ensures that default_value fits max_value_length.
			res.values = append(res.values, kv{Key: attr.key, Value: attr.defaultValue})
		case attr.optional:
			// Omitted.
		default:
			if err != nil && errorMode == ottl.PropagateError {
				return factsResult{truncated: res.truncated}, fmt.Errorf("rule %q: failed to evaluate required attribute %q: %w", r.cfg.Name, attr.key, err)
			}
			return factsResult{missing: attr.key, missingErr: err, truncated: res.truncated}, nil
		}
	}
	return res, nil
}

// appendKeyPart appends one identity value to a state key: its length,
// a colon, then the value. encodeIdentity uses the same format.
func appendKeyPart(buf []byte, v string) []byte {
	buf = strconv.AppendInt(buf, int64(len(v)), 10)
	buf = append(buf, ':')
	return append(buf, v...)
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

// valueString converts the result of a value expression to a string with
// pcommon.Value.AsString semantics: int 4 is "4", bool true is "true", and
// maps and slices are JSON. Types that pcommon cannot hold, for example
// time.Time, use their fmt representation. nil is "".
func valueString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case pcommon.Value:
		return t.AsString()
	case pcommon.Map:
		val := pcommon.NewValueMap()
		t.CopyTo(val.Map())
		return val.AsString()
	case pcommon.Slice:
		val := pcommon.NewValueSlice()
		t.CopyTo(val.Slice())
		return val.AsString()
	}
	val := pcommon.NewValueEmpty()
	if err := val.FromRaw(v); err != nil {
		return fmt.Sprint(v)
	}
	return val.AsString()
}

// fitValue returns a value as the connector stores and emits it: valid
// UTF-8 of at most n bytes. It replaces each run of invalid UTF-8 bytes
// with U+FFFD (validUTF8), then cuts the result to n bytes (truncateUTF8).
// The replacement comes first because it can make the value longer, and
// the cut of a valid string never splits a character, so the result is
// valid and within n. A valid value that fits n is returned as is.
func fitValue(s string, n int) string {
	return truncateUTF8(validUTF8(s), n)
}

// fitCounted is fitValue that adds 1 to *count when the cut shortens the
// value.
func fitCounted(s string, n int, count *int) string {
	v := validUTF8(s)
	t := truncateUTF8(v, n)
	if len(t) < len(v) {
		*count++
	}
	return t
}

// validUTF8 returns s with each run of invalid UTF-8 bytes replaced by
// U+FFFD. A receiver can pass bytes that are not valid UTF-8, for example
// the filelog receiver with encoding utf8-raw. A string attribute that is not
// valid UTF-8 is not a valid protobuf string, and a strict receiver can
// refuse the whole export request that holds it. A valid s is checked without allocating and returned as is.
func validUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "\uFFFD")
}

// truncateUTF8 returns s cut to at most n bytes without splitting a UTF-8
// sequence: if the cut falls inside a multi-byte character, it moves back to
// the start of that character. Invalid UTF-8 is cut at n.
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// s[n] is the first byte that does not fit. In valid UTF-8 the start of
	// its character is at most utf8.UTFMax-1 bytes earlier.
	for cut := n; cut >= 0 && cut > n-utf8.UTFMax; cut-- {
		if utf8.RuneStart(s[cut]) {
			return s[:cut]
		}
	}
	return s[:n]
}
