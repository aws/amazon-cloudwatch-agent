// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"cmp"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// Connector-level defaults.
const (
	defaultEmitInterval   = 60 * time.Second
	defaultEmitOnChange   = true
	defaultMaxStateBytes  = 1 << 20 // 1 MiB
	defaultMaxValueLength = 256
	defaultErrorMode      = ottl.IgnoreError
)

// Rule-level defaults, used when a rule leaves the field at 0.
const (
	defaultMaxEntities   = 500
	defaultMaxAttributes = 32
)

// Bounds enforced by Validate.
const (
	minEmitInterval = time.Second
	maxEmitInterval = 24 * time.Hour

	minMaxStateBytes = 64 << 10 // 64 KiB
	maxMaxStateBytes = 64 << 20 // 64 MiB

	minMaxValueLength = 1
	// maxMaxValueLength is the CloudWatch limit on the length of a label value.
	maxMaxValueLength = 1024

	minMaxAttributes = 1
	maxMaxAttributes = 128

	minMaxEntities = 1
	maxMaxEntities = 100_000

	// maxRuleNameLength bounds the rule name, which is also the metric name.
	maxRuleNameLength = 255

	// maxLabelsPerDatapoint is the CloudWatch limit on labels per data point.
	maxLabelsPerDatapoint = 150
)

// Config configures the signal_to_info_metric connector. The connector
// consumes logs, matches log records against Rules, keeps the latest facts
// per entity, and emits one Info metric (a gauge with value 1 whose data
// point attributes carry the facts) per rule and entity.
type Config struct {
	// Storage is the ID of a storage extension used to persist state across
	// restarts. When nil, state is kept in memory only. It is also kept in
	// memory only when the extension cannot give a storage client at Start.
	Storage *component.ID `mapstructure:"storage"`
	// EmitInterval is how often the current Info metrics are re-emitted.
	// Rules can override it.
	EmitInterval time.Duration `mapstructure:"emit_interval"`
	// EmitOnChange emits an Info metric as soon as an entity's facts change,
	// in addition to the periodic emission.
	EmitOnChange bool `mapstructure:"emit_on_change"`
	// MaxStateBytes bounds the size of the serialized state.
	MaxStateBytes int `mapstructure:"max_state_bytes"`
	// MaxValueLength bounds the length of every extracted attribute value.
	MaxValueLength int `mapstructure:"max_value_length"`
	// ErrorMode determines how runtime OTTL evaluation errors are handled:
	// propagate, ignore, or silent. It does not affect configuration-time
	// parse errors, which always fail validation. The default is ignore so
	// that a bad record never fails the logs pipeline the connector shares
	// with other exporters.
	ErrorMode ottl.ErrorMode `mapstructure:"error_mode"`
	// Rules define which log records produce which Info metric.
	Rules []RuleConfig `mapstructure:"rules"`
}

// RuleConfig defines one Info metric and how to extract its facts from logs.
type RuleConfig struct {
	// Name is the metric name and the key of the rule's state.
	Name        string `mapstructure:"name"`
	Description string `mapstructure:"description"`
	Unit        string `mapstructure:"unit"`
	// BodyContains lists substrings. If any are set, a record matches only if
	// its body contains at least one of them. This is a plain substring check
	// on a string body, with no OTTL and no regex. It runs before Conditions.
	BodyContains []string `mapstructure:"body_contains"`
	// Conditions are OTTL conditions, ORed. If any are set, a record matches
	// only if at least one condition is true. A rule needs BodyContains,
	// Conditions, or both. If both are set, a record must pass both checks.
	Conditions []string `mapstructure:"conditions"`
	// Identity lists the resource attribute keys that identify an entity,
	// for example k8s.pod.uid. The connector keeps the latest facts per
	// distinct identity.
	Identity []string `mapstructure:"identity"`
	// IncludeResourceAttributes lists resource attribute keys copied to the
	// emitted metric's resource.
	IncludeResourceAttributes []string `mapstructure:"include_resource_attributes"`
	// Attributes are the facts extracted from a matching record and emitted
	// as data point attributes.
	Attributes []AttributeConfig `mapstructure:"attributes"`
	// EmitInterval overrides the connector emit_interval for this rule.
	// 0 uses the connector value.
	EmitInterval time.Duration `mapstructure:"emit_interval"`
	// MaxEntities bounds the number of entities tracked for this rule.
	// 0 uses defaultMaxEntities.
	MaxEntities int `mapstructure:"max_entities"`
	// MaxAttributes bounds the number of attributes this rule may define.
	// 0 uses defaultMaxAttributes.
	MaxAttributes int `mapstructure:"max_attributes"`
}

// AttributeConfig defines one fact extracted from a matching log record.
type AttributeConfig struct {
	// Key is the data point attribute key.
	Key string `mapstructure:"key"`
	// Value is an OTTL value expression. When empty, the value is read from
	// log.attributes["<key>"].
	Value string `mapstructure:"value"`
	// DefaultValue is used when the value expression returns an error, nil,
	// or an empty string. When empty, Optional decides what happens.
	DefaultValue string `mapstructure:"default_value"`
	// Optional allows a matching record to yield no value for this fact, in
	// which case the attribute is omitted. When false, a record that yields
	// no value for it is not used. It cannot be combined with DefaultValue.
	Optional bool `mapstructure:"optional"`
}

var _ component.Config = (*Config)(nil)

// Validate checks the configuration and reports every problem it finds.
// It parses all OTTL conditions and value expressions, so a configuration
// that validates also compiles at runtime.
func (cfg *Config) Validate() error {
	var errs []error

	if !inRange(cfg.EmitInterval, minEmitInterval, maxEmitInterval) {
		errs = append(errs, fmt.Errorf("emit_interval must be between %s and %s, got %s",
			minEmitInterval, maxEmitInterval, cfg.EmitInterval))
	}
	if !inRange(cfg.MaxStateBytes, minMaxStateBytes, maxMaxStateBytes) {
		errs = append(errs, fmt.Errorf("max_state_bytes must be between %d and %d, got %d",
			minMaxStateBytes, maxMaxStateBytes, cfg.MaxStateBytes))
	}
	if !inRange(cfg.MaxValueLength, minMaxValueLength, maxMaxValueLength) {
		errs = append(errs, fmt.Errorf("max_value_length must be between %d and %d, got %d",
			minMaxValueLength, maxMaxValueLength, cfg.MaxValueLength))
	}
	switch cfg.ErrorMode {
	case ottl.PropagateError, ottl.IgnoreError, ottl.SilentError:
	default:
		errs = append(errs, fmt.Errorf("error_mode must be one of %q, %q, or %q, got %q",
			ottl.PropagateError, ottl.IgnoreError, ottl.SilentError, cfg.ErrorMode))
	}

	if len(cfg.Rules) == 0 {
		errs = append(errs, errors.New("at least one rule must be configured"))
		return errors.Join(errs...)
	}

	parser, err := newLogParser(component.TelemetrySettings{Logger: zap.NewNop()})
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to create OTTL log parser: %w", err))
		return errors.Join(errs...)
	}

	firstRuleWithName := make(map[string]int, len(cfg.Rules))
	for i := range cfg.Rules {
		rule := &cfg.Rules[i]
		var ruleErrs []error
		if rule.Name != "" {
			if j, dup := firstRuleWithName[rule.Name]; dup {
				ruleErrs = append(ruleErrs, fmt.Errorf("duplicate rule name, already used by rules[%d]", j))
			} else {
				firstRuleWithName[rule.Name] = i
			}
		}
		ruleErrs = append(ruleErrs, rule.validate(&parser, cfg.MaxValueLength)...)
		for _, e := range ruleErrs {
			errs = append(errs, fmt.Errorf("%s: %w", rule.label(i), e))
		}
	}

	return errors.Join(errs...)
}

// validate checks a single rule. It is not named Validate so that
// xconfmap.Validate does not run it a second time. maxValueLength is the
// connector max_value_length.
func (r *RuleConfig) validate(parser *ottl.Parser[*ottllog.TransformContext], maxValueLength int) []error {
	var errs []error

	switch {
	case r.Name == "":
		errs = append(errs, errors.New("name must not be empty"))
	case len(r.Name) > maxRuleNameLength:
		errs = append(errs, fmt.Errorf("name must be at most %d bytes, got %d", maxRuleNameLength, len(r.Name)))
	}

	if r.EmitInterval != 0 && !inRange(r.EmitInterval, minEmitInterval, maxEmitInterval) {
		errs = append(errs, fmt.Errorf("emit_interval must be 0 (use the connector emit_interval) or between %s and %s, got %s",
			minEmitInterval, maxEmitInterval, r.EmitInterval))
	}
	if r.MaxEntities != 0 && !inRange(r.MaxEntities, minMaxEntities, maxMaxEntities) {
		errs = append(errs, fmt.Errorf("max_entities must be 0 (default %d) or between %d and %d, got %d",
			defaultMaxEntities, minMaxEntities, maxMaxEntities, r.MaxEntities))
	}
	if r.MaxAttributes != 0 && !inRange(r.MaxAttributes, minMaxAttributes, maxMaxAttributes) {
		errs = append(errs, fmt.Errorf("max_attributes must be 0 (default %d) or between %d and %d, got %d",
			defaultMaxAttributes, minMaxAttributes, maxMaxAttributes, r.MaxAttributes))
	}

	for i, s := range r.BodyContains {
		if s == "" {
			errs = append(errs, fmt.Errorf("body_contains[%d] must not be empty", i))
		}
	}

	if len(r.BodyContains) == 0 && len(r.Conditions) == 0 {
		errs = append(errs, errors.New("at least one of body_contains or conditions is required"))
	}
	if len(r.Conditions) > 0 {
		if _, err := parser.ParseConditions(r.Conditions); err != nil {
			errs = append(errs, fmt.Errorf("conditions: %w", err))
		}
	}

	if len(r.Identity) == 0 {
		errs = append(errs, errors.New("identity must list at least one resource attribute key"))
	}
	errs = append(errs, validateKeyList("identity", r.Identity)...)
	errs = append(errs, validateKeyList("include_resource_attributes", r.IncludeResourceAttributes)...)
	// The emitted resource has only the included resource attributes, so
	// every identity key must be one of them. Otherwise two entities that
	// differ only in that key give the same series.
	included := make(map[string]struct{}, len(r.IncludeResourceAttributes))
	for _, k := range r.IncludeResourceAttributes {
		included[k] = struct{}{}
	}
	reported := make(map[string]struct{}, len(r.Identity))
	for _, k := range r.Identity {
		if k == "" {
			continue
		}
		_, ok := included[k]
		_, done := reported[k]
		if !ok && !done {
			reported[k] = struct{}{}
			errs = append(errs, fmt.Errorf("identity key %q must also be listed in include_resource_attributes, "+
				"so that two entities never give the same series", k))
		}
	}

	if len(r.Attributes) == 0 {
		errs = append(errs, errors.New("at least one attribute is required"))
	}
	if maxAttrs := r.effectiveMaxAttributes(); len(r.Attributes) > maxAttrs {
		errs = append(errs, fmt.Errorf("defines %d attributes, more than max_attributes (%d)", len(r.Attributes), maxAttrs))
	}
	firstAttrWithKey := make(map[string]int, len(r.Attributes))
	for i := range r.Attributes {
		attr := &r.Attributes[i]
		if attr.Key == "" {
			errs = append(errs, fmt.Errorf("attributes[%d]: key must not be empty", i))
		} else if j, dup := firstAttrWithKey[attr.Key]; dup {
			errs = append(errs, fmt.Errorf("attributes[%d]: duplicate key %q, already used by attributes[%d]", i, attr.Key, j))
		} else {
			firstAttrWithKey[attr.Key] = i
		}
		if attr.Optional && attr.DefaultValue != "" {
			errs = append(errs, fmt.Errorf("attributes[%d]: optional has no effect when default_value is set", i))
		}
		if len(attr.DefaultValue) > maxValueLength {
			errs = append(errs, fmt.Errorf("attributes[%d]: default_value is %d bytes, longer than max_value_length (%d)",
				i, len(attr.DefaultValue), maxValueLength))
		}
		if attr.Key == "" && attr.Value == "" {
			continue
		}
		expr := attr.valueExpression()
		if _, err := parser.ParseValueExpression(expr); err != nil {
			errs = append(errs, fmt.Errorf("attributes[%d]: unable to parse OTTL value expression %q: %w", i, expr, err))
		}
	}

	if n := r.labelCount(); n > maxLabelsPerDatapoint {
		errs = append(errs, fmt.Errorf("would emit %d labels per data point (attributes + include_resource_attributes),"+
			" more than the CloudWatch limit of %d", n, maxLabelsPerDatapoint))
	}

	return errs
}

// label identifies a rule in error messages.
func (r *RuleConfig) label(index int) string {
	if r.Name == "" {
		return fmt.Sprintf("rules[%d]", index)
	}
	return fmt.Sprintf("rules[%d] (%s)", index, r.Name)
}

// effectiveEmitInterval returns the rule's emit interval, or def (the
// connector emit_interval) when the rule does not set one.
func (r *RuleConfig) effectiveEmitInterval(def time.Duration) time.Duration {
	if r.EmitInterval > 0 {
		return r.EmitInterval
	}
	return def
}

// effectiveMaxEntities returns the rule's entity limit, applying the default.
func (r *RuleConfig) effectiveMaxEntities() int {
	if r.MaxEntities > 0 {
		return r.MaxEntities
	}
	return defaultMaxEntities
}

// effectiveMaxAttributes returns the rule's attribute limit, applying the default.
func (r *RuleConfig) effectiveMaxAttributes() int {
	if r.MaxAttributes > 0 {
		return r.MaxAttributes
	}
	return defaultMaxAttributes
}

// labelCount returns the number of labels each emitted data point carries:
// the extracted attributes and the included resource attributes.
func (r *RuleConfig) labelCount() int {
	return len(r.Attributes) + len(r.IncludeResourceAttributes)
}

// valueExpression returns the OTTL value expression for the attribute,
// defaulting to log.attributes["<key>"] when Value is empty.
func (a *AttributeConfig) valueExpression() string {
	if a.Value != "" {
		return a.Value
	}
	return "log.attributes[" + strconv.Quote(a.Key) + "]"
}

// newLogParser returns the OTTL parser for rule conditions and attribute
// value expressions. Only converters are registered, because rules contain
// conditions and value expressions, never editor statements.
func newLogParser(set component.TelemetrySettings) (ottl.Parser[*ottllog.TransformContext], error) {
	return ottllog.NewParser(ottlfuncs.StandardConverters[*ottllog.TransformContext](), set)
}

// validateKeyList reports empty and duplicate entries in a list of keys.
func validateKeyList(field string, keys []string) []error {
	var errs []error
	first := make(map[string]int, len(keys))
	for i, k := range keys {
		if k == "" {
			errs = append(errs, fmt.Errorf("%s[%d] must not be empty", field, i))
			continue
		}
		if j, dup := first[k]; dup {
			errs = append(errs, fmt.Errorf("%s[%d]: duplicate key %q, already listed at %s[%d]", field, i, k, field, j))
			continue
		}
		first[k] = i
	}
	return errs
}

func inRange[T cmp.Ordered](v, lo, hi T) bool {
	return v >= lo && v <= hi
}
