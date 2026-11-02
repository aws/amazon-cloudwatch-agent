// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"cmp"
	"errors"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

const (
	defaultInterval       = time.Hour
	defaultSourceFile     = "log.file.path"
	defaultMaxStateBytes  = 4 << 20  // 4 MiB
	defaultMaxRecordBytes = 64 << 10 // 64 KiB
	defaultErrorMode      = ottl.IgnoreError
)

const defaultMaxEntries = 500

const (
	minInterval = time.Minute
	maxInterval = 24 * time.Hour

	minMaxStateBytes = 64 << 10 // 64 KiB
	maxMaxStateBytes = 64 << 20 // 64 MiB

	minMaxRecordBytes = 1 << 10 // 1 KiB
	maxMaxRecordBytes = 1 << 20 // 1 MiB

	minMaxEntries = 1
	maxMaxEntries = 100_000

	maxRuleNameLength = 255
)

type Config struct {
	Storage             *component.ID  `mapstructure:"storage"`
	Interval            time.Duration  `mapstructure:"interval"`
	SourceFileAttribute string         `mapstructure:"source_file_attribute"`
	MaxStateBytes       int            `mapstructure:"max_state_bytes"`
	MaxRecordBytes      int            `mapstructure:"max_record_bytes"`
	ErrorMode           ottl.ErrorMode `mapstructure:"error_mode"`
	Rules               []RuleConfig   `mapstructure:"rules"`
}

type RuleConfig struct {
	Name         string   `mapstructure:"name"`
	BodyContains []string `mapstructure:"body_contains"`
	Conditions   []string `mapstructure:"conditions"`
	Identity     []string `mapstructure:"identity"`
	// MaxEntries 0 uses defaultMaxEntries.
	MaxEntries int `mapstructure:"max_entries"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	var errs []error

	if !inRange(cfg.Interval, minInterval, maxInterval) {
		errs = append(errs, fmt.Errorf("interval must be between %s and %s, got %s",
			minInterval, maxInterval, cfg.Interval))
	}
	if cfg.SourceFileAttribute == "" {
		errs = append(errs, errors.New("source_file_attribute must not be empty: the connector removes a kept record "+
			"when its source file is gone, and it has no other sign that the source stopped"))
	}
	if !inRange(cfg.MaxStateBytes, minMaxStateBytes, maxMaxStateBytes) {
		errs = append(errs, fmt.Errorf("max_state_bytes must be between %d and %d, got %d",
			minMaxStateBytes, maxMaxStateBytes, cfg.MaxStateBytes))
	}
	if !inRange(cfg.MaxRecordBytes, minMaxRecordBytes, maxMaxRecordBytes) {
		errs = append(errs, fmt.Errorf("max_record_bytes must be between %d and %d, got %d",
			minMaxRecordBytes, maxMaxRecordBytes, cfg.MaxRecordBytes))
	} else if cfg.MaxRecordBytes > cfg.MaxStateBytes {
		errs = append(errs, fmt.Errorf("max_record_bytes (%d) must not be larger than max_state_bytes (%d)",
			cfg.MaxRecordBytes, cfg.MaxStateBytes))
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
		ruleErrs = append(ruleErrs, rule.validate(&parser)...)
		for _, e := range ruleErrs {
			errs = append(errs, fmt.Errorf("%s: %w", rule.label(i), e))
		}
	}

	return errors.Join(errs...)
}

// validate is not named Validate, so that xconfmap.Validate does not run it twice.
func (r *RuleConfig) validate(parser *ottl.Parser[*ottllog.TransformContext]) []error {
	var errs []error

	switch {
	case r.Name == "":
		errs = append(errs, errors.New("name must not be empty"))
	case len(r.Name) > maxRuleNameLength:
		errs = append(errs, fmt.Errorf("name must be at most %d bytes, got %d", maxRuleNameLength, len(r.Name)))
	}

	if r.MaxEntries != 0 && !inRange(r.MaxEntries, minMaxEntries, maxMaxEntries) {
		errs = append(errs, fmt.Errorf("max_entries must be 0 (default %d) or between %d and %d, got %d",
			defaultMaxEntries, minMaxEntries, maxMaxEntries, r.MaxEntries))
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
	first := make(map[string]int, len(r.Identity))
	for i, k := range r.Identity {
		if k == "" {
			errs = append(errs, fmt.Errorf("identity[%d] must not be empty", i))
			continue
		}
		if j, dup := first[k]; dup {
			errs = append(errs, fmt.Errorf("identity[%d]: duplicate key %q, already listed at identity[%d]", i, k, j))
			continue
		}
		first[k] = i
	}

	return errs
}

func (r *RuleConfig) label(index int) string {
	if r.Name == "" {
		return fmt.Sprintf("rules[%d]", index)
	}
	return fmt.Sprintf("rules[%d] (%s)", index, r.Name)
}

func (r *RuleConfig) effectiveMaxEntries() int {
	if r.MaxEntries > 0 {
		return r.MaxEntries
	}
	return defaultMaxEntries
}

// newLogParser registers converters only, because rules hold conditions.
func newLogParser(set component.TelemetrySettings) (ottl.Parser[*ottllog.TransformContext], error) {
	return ottllog.NewParser(ottlfuncs.StandardConverters[*ottllog.TransformContext](), set)
}

func inRange[T cmp.Ordered](v, lo, hi T) bool {
	return v >= lo && v <= hi
}
