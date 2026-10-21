// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package signaltoinfometricconnector

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/pdata/plog"
)

// baseRule matches the &base_rule anchor in testdata/config.yaml.
func baseRule() RuleConfig {
	return RuleConfig{
		Name:                      "example_info",
		Conditions:                []string{`body == "service started"`},
		Identity:                  []string{"k8s.pod.uid"},
		IncludeResourceAttributes: []string{"k8s.pod.uid", "k8s.pod.name"},
		Attributes:                []AttributeConfig{{Key: "version"}},
	}
}

func validConfig() *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.Rules = []RuleConfig{baseRule()}
	return cfg
}

func TestLoadConfig(t *testing.T) {
	storageID := component.MustNewIDWithName("file_storage", "state")
	limitsRule := baseRule()
	limitsRule.Name = strings.Repeat("n", maxRuleNameLength)
	limitsRule.EmitInterval = maxEmitInterval
	limitsRule.MaxEntities = maxMaxEntities
	limitsRule.MaxAttributes = maxMaxAttributes
	lowerLimitsRule := baseRule()
	lowerLimitsRule.Name = "lower_limits_info"
	lowerLimitsRule.MaxEntities = minMaxEntities
	lowerLimitsRule.MaxAttributes = minMaxAttributes

	testCases := []struct {
		name string
		// want is the expected configuration of a valid case.
		want *Config
		// unmarshalErr is a substring of the expected unmarshal error.
		unmarshalErr string
		// errContains lists substrings of the expected validation error.
		errContains []string
		// errCount is the number of errors Validate reports. 0 means len(errContains).
		errCount int
	}{
		{
			name: "vllm",
			want: &Config{
				Storage:        &storageID,
				EmitInterval:   30 * time.Second,
				EmitOnChange:   true,
				MaxStateBytes:  defaultMaxStateBytes,
				MaxValueLength: defaultMaxValueLength,
				ErrorMode:      ottl.IgnoreError,
				Rules: []RuleConfig{{
					Name:        "vllm_engine_config_info",
					Description: "vLLM engine configuration from the engine startup log line.",
					BodyContains: []string{
						"Initializing an LLM engine (v",
						"Initializing a V0 LLM engine (v",
						"Initializing a V1 LLM engine (v",
					},
					Identity:                  []string{"k8s.namespace.name", "k8s.pod.name", "k8s.container.name"},
					IncludeResourceAttributes: []string{"k8s.namespace.name", "k8s.pod.name", "k8s.container.name", "k8s.pod.uid"},
					Attributes: []AttributeConfig{
						{Key: "vllm_version", Value: `ExtractPatterns(log.body, "Initializing an? (?:V[01] )?LLM engine \\(v(?P<v>[^)]+)\\) with config: ")["v"]`},
						{Key: "vllm_engine", Value: `ExtractPatterns(log.body, "Initializing a (?P<v>V[01]) LLM engine")["v"]`, DefaultValue: "V0"},
						{Key: "tensor_parallel_size", Value: `ExtractPatterns(log.body, ", tensor_parallel_size=(?P<v>\\d+),")["v"]`, Optional: true},
					},
				}},
			},
		},
		{
			name: "minimal",
			want: validConfig(),
		},
		{
			name: "body_contains_only",
			want: func() *Config {
				cfg := validConfig()
				cfg.Rules[0].BodyContains = []string{"service started"}
				cfg.Rules[0].Conditions = []string{}
				return cfg
			}(),
		},
		{
			name: "limits",
			want: &Config{
				EmitInterval:   minEmitInterval,
				EmitOnChange:   false,
				MaxStateBytes:  maxMaxStateBytes,
				MaxValueLength: maxMaxValueLength,
				ErrorMode:      ottl.PropagateError,
				Rules:          []RuleConfig{limitsRule, lowerLimitsRule},
			},
		},
		{name: "no_rules", errContains: []string{"at least one rule must be configured"}},
		{name: "emit_interval_too_small", errContains: []string{"emit_interval must be between 1s and 24h0m0s, got 999ms"}},
		{name: "emit_interval_too_large", errContains: []string{"emit_interval must be between 1s and 24h0m0s, got 24h0m1s"}},
		{name: "emit_interval_negative", errContains: []string{"emit_interval must be between 1s and 24h0m0s, got -1s"}},
		{name: "max_state_bytes_too_small", errContains: []string{"max_state_bytes must be between 65536 and 67108864, got 65535"}},
		{name: "max_state_bytes_too_large", errContains: []string{"max_state_bytes must be between 65536 and 67108864, got 67108865"}},
		{name: "max_value_length_zero", errContains: []string{"max_value_length must be between 1 and 1024, got 0"}},
		{name: "max_value_length_too_large", errContains: []string{"max_value_length must be between 1 and 1024, got 1025"}},
		{name: "invalid_error_mode", unmarshalErr: "unknown error mode explode"},
		{name: "empty_rule_name", errContains: []string{"rules[0]: name must not be empty"}},
		{name: "long_rule_name", errContains: []string{"name must be at most 255 bytes, got 256"}},
		{name: "duplicate_rule_name", errContains: []string{"rules[1] (example_info): duplicate rule name, already used by rules[0]"}},
		{name: "no_body_contains_or_conditions", errContains: []string{"rules[0] (example_info): at least one of body_contains or conditions is required"}},
		{
			name: "invalid_condition",
			errContains: []string{
				`rules[0] (example_info): conditions: unable to parse OTTL condition "IsMatch(body, "`,
				`unable to parse OTTL condition "NoSuchConverter(body)"`,
			},
			errCount: 1,
		},
		{name: "empty_body_contains", errContains: []string{"rules[0] (example_info): body_contains[1] must not be empty"}},
		{name: "no_identity", errContains: []string{"rules[0] (example_info): identity must list at least one resource attribute key"}},
		{name: "empty_identity_key", errContains: []string{"rules[0] (example_info): identity[1] must not be empty"}},
		{name: "duplicate_identity_key", errContains: []string{`identity[1]: duplicate key "k8s.pod.uid", already listed at identity[0]`}},
		{name: "identity_not_included", errContains: []string{`rules[0] (example_info): identity key "k8s.pod.uid" must also be listed in include_resource_attributes, so that two entities never give the same series`}},
		{name: "empty_include_key", errContains: []string{"rules[0] (example_info): include_resource_attributes[1] must not be empty"}},
		{name: "duplicate_include_key", errContains: []string{`include_resource_attributes[2]: duplicate key "k8s.pod.name", already listed at include_resource_attributes[1]`}},
		{name: "no_attributes", errContains: []string{"rules[0] (example_info): at least one attribute is required"}},
		{name: "empty_attribute_key", errContains: []string{"rules[0] (example_info): attributes[0]: key must not be empty"}},
		{name: "duplicate_attribute_key", errContains: []string{`attributes[1]: duplicate key "version", already used by attributes[0]`}},
		{
			name: "invalid_attribute_value",
			errContains: []string{
				`attributes[0]: unable to parse OTTL value expression "ExtractPatterns(body, \"(?P<v>[0-9\")[\"v\"]"`,
				"is not a valid pattern",
				`attributes[1]: unable to parse OTTL value expression "log.no_such_field"`,
			},
			errCount: 2,
		},
		{name: "optional_with_default_value", errContains: []string{"rules[0] (example_info): attributes[0]: optional has no effect when default_value is set"}},
		{name: "default_value_too_long", errContains: []string{"rules[0] (example_info): attributes[0]: default_value is 3 bytes, longer than max_value_length (2)"}},
		{name: "too_many_attributes", errContains: []string{"rules[0] (example_info): defines 2 attributes, more than max_attributes (1)"}},
		{name: "rule_emit_interval_negative", errContains: []string{"emit_interval must be 0 (use the connector emit_interval) or between 1s and 24h0m0s, got -1s"}},
		{name: "rule_emit_interval_too_small", errContains: []string{"emit_interval must be 0 (use the connector emit_interval) or between 1s and 24h0m0s, got 500ms"}},
		{name: "rule_emit_interval_too_large", errContains: []string{"emit_interval must be 0 (use the connector emit_interval) or between 1s and 24h0m0s, got 25h0m0s"}},
		{name: "max_entities_negative", errContains: []string{"max_entities must be 0 (default 500) or between 1 and 100000, got -1"}},
		{name: "max_entities_too_large", errContains: []string{"max_entities must be 0 (default 500) or between 1 and 100000, got 100001"}},
		{name: "max_attributes_negative", errContains: []string{"max_attributes must be 0 (default 32) or between 1 and 128, got -1"}},
		{name: "max_attributes_too_large", errContains: []string{"max_attributes must be 0 (default 32) or between 1 and 128, got 129"}},
		{
			name: "multiple_errors",
			errContains: []string{
				"max_value_length must be between 1 and 1024, got 2048",
				"rules[0]: name must not be empty",
				"rules[0]: at least one of body_contains or conditions is required",
				"rules[0]: at least one attribute is required",
				"rules[1] (example_info): identity must list at least one resource attribute key",
			},
		},
	}

	conf, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id := component.NewIDWithName(component.MustNewType(TypeStr), tc.name)
			sub, err := conf.Sub(id.String())
			require.NoError(t, err)
			require.NotEmpty(t, sub.AllKeys(), "missing %s in testdata/config.yaml", id)

			cfg := NewFactory().CreateDefaultConfig()
			err = sub.Unmarshal(cfg)
			if tc.unmarshalErr != "" {
				require.ErrorContains(t, err, tc.unmarshalErr)
				return
			}
			require.NoError(t, err)

			err = xconfmap.Validate(cfg)
			if len(tc.errContains) == 0 {
				require.NoError(t, err)
				assert.Equal(t, tc.want, cfg)
				return
			}
			require.Error(t, err)
			for _, want := range tc.errContains {
				assert.ErrorContains(t, err, want)
			}
			wantCount := tc.errCount
			if wantCount == 0 {
				wantCount = len(tc.errContains)
			}
			assert.Len(t, joinedErrors(cfg.(*Config).Validate()), wantCount, "all errors: %v", err)
		})
	}
}

// joinedErrors returns the errors joined by errors.Join, or err itself.
func joinedErrors(err error) []error {
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		return joined.Unwrap()
	}
	if err == nil {
		return nil
	}
	return []error{err}
}

func TestValidateErrorMode(t *testing.T) {
	for _, mode := range []ottl.ErrorMode{ottl.PropagateError, ottl.IgnoreError, ottl.SilentError} {
		cfg := validConfig()
		cfg.ErrorMode = mode
		assert.NoError(t, cfg.Validate(), mode)
	}
	for _, mode := range []ottl.ErrorMode{"", "explode"} {
		cfg := validConfig()
		cfg.ErrorMode = mode
		assert.ErrorContains(t, cfg.Validate(), `error_mode must be one of "propagate", "ignore", or "silent"`, mode)
	}
}

func TestValidateDefaultValueLength(t *testing.T) {
	cfg := validConfig()
	cfg.MaxValueLength = 4
	// "é" is 2 bytes, so the limit counts bytes, not characters.
	cfg.Rules[0].Attributes[0].DefaultValue = "aaé"
	assert.NoError(t, cfg.Validate())
	cfg.Rules[0].Attributes[0].DefaultValue = "aaaé"
	assert.ErrorContains(t, cfg.Validate(), "attributes[0]: default_value is 5 bytes, longer than max_value_length (4)")
}

func TestValidateLabelBudget(t *testing.T) {
	// ruleWithLabels returns a rule with numAttrs attributes and numIncluded
	// included resource attributes, the first of which is the identity key.
	ruleWithLabels := func(numAttrs, numIncluded int) RuleConfig {
		rule := baseRule()
		rule.Attributes = nil
		for i := 0; i < numAttrs; i++ {
			rule.Attributes = append(rule.Attributes, AttributeConfig{Key: fmt.Sprintf("attr%d", i)})
		}
		rule.IncludeResourceAttributes = []string{"k8s.pod.uid"}
		for i := 1; i < numIncluded; i++ {
			rule.IncludeResourceAttributes = append(rule.IncludeResourceAttributes, fmt.Sprintf("res%d", i))
		}
		return rule
	}
	testCases := []struct {
		name       string
		attrs      int
		included   int
		wantErr    bool
		wantLabels int
	}{
		{name: "at_limit", attrs: 32, included: 118, wantLabels: 150},
		{name: "over_limit", attrs: 32, included: 119, wantLabels: 151, wantErr: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Rules = []RuleConfig{ruleWithLabels(tc.attrs, tc.included)}
			assert.Equal(t, tc.wantLabels, cfg.Rules[0].labelCount())
			err := cfg.Validate()
			if !tc.wantErr {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, fmt.Sprintf("would emit %d labels per data point", tc.wantLabels))
			assert.ErrorContains(t, err, "more than the CloudWatch limit of 150")
			assert.Len(t, joinedErrors(err), 1)
		})
	}
}

func TestRuleDefaults(t *testing.T) {
	var rule RuleConfig
	assert.Equal(t, defaultEmitInterval, rule.effectiveEmitInterval(defaultEmitInterval))
	assert.Equal(t, defaultMaxEntities, rule.effectiveMaxEntities())
	assert.Equal(t, defaultMaxAttributes, rule.effectiveMaxAttributes())

	rule = RuleConfig{EmitInterval: 5 * time.Minute, MaxEntities: 7, MaxAttributes: 3}
	assert.Equal(t, 5*time.Minute, rule.effectiveEmitInterval(defaultEmitInterval))
	assert.Equal(t, 7, rule.effectiveMaxEntities())
	assert.Equal(t, 3, rule.effectiveMaxAttributes())
}

func TestAttributeDefaultValueExpression(t *testing.T) {
	for _, key := range []string{"vllm_version", "k8s.pod.name", `quote"and\backslash`, "unicode-é"} {
		attr := AttributeConfig{Key: key}
		assert.Equal(t, `log.attributes["`+strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(key)+`"]`, attr.valueExpression())
		lr := plog.NewLogRecord()
		lr.Attributes().PutStr(key, "value-of-"+key)
		got := evalValue(t, attr.valueExpression(), plog.NewResourceLogs(), lr)
		assert.Equal(t, "value-of-"+key, got, key)
	}
	attr := AttributeConfig{Key: "k", Value: `log.attributes["other"]`}
	assert.Equal(t, `log.attributes["other"]`, attr.valueExpression())
}

// TestOTTLPathStyles shows that both the short ("body") and the
// context-qualified ("log.body") path styles parse and read the same data.
func TestOTTLPathStyles(t *testing.T) {
	rl := plog.NewResourceLogs()
	rl.Resource().Attributes().PutStr("k8s.pod.uid", "uid-1")
	lr := plog.NewLogRecord()
	lr.Body().SetStr("service started")
	lr.Attributes().PutStr("level", "info")

	for _, cond := range []string{
		`IsMatch(body, "started")`,
		`IsMatch(log.body, "started")`,
		`attributes["level"] == "info"`,
		`log.attributes["level"] == "info"`,
		`resource.attributes["k8s.pod.uid"] == "uid-1"`,
	} {
		assert.True(t, evalCondition(t, cond, rl, lr), cond)
	}
	for expr, want := range map[string]any{
		`body`:                               "service started",
		`log.body`:                           "service started",
		`attributes["level"]`:                "info",
		`log.attributes["level"]`:            "info",
		`resource.attributes["k8s.pod.uid"]`: "uid-1",
	} {
		assert.Equal(t, want, evalValue(t, expr, rl, lr), expr)
	}
}

// loadVLLMRule returns the vLLM rule from testdata/config.yaml.
func loadVLLMRule(t *testing.T) RuleConfig {
	t.Helper()
	conf, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	sub, err := conf.Sub(component.NewIDWithName(component.MustNewType(TypeStr), "vllm").String())
	require.NoError(t, err)
	cfg := createDefaultConfig().(*Config)
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, cfg.Validate())
	return cfg.Rules[0]
}

// TestVLLMRuleExpressions evaluates the vLLM rule from testdata against
// sample engine startup lines from the V1 and V0 engines.
func TestVLLMRuleExpressions(t *testing.T) {
	rule := loadVLLMRule(t)
	require.Empty(t, rule.Conditions, "the vLLM rule matches with body_contains only")

	testCases := []struct {
		name  string
		body  string
		match bool
		want  map[string]any
		// wantErr maps attribute keys to the expected evaluation error.
		wantErr map[string]string
		// noSeries is set when a required fact without default_value fails,
		// so the runtime does not use the record.
		noSeries bool
	}{
		{
			name:  "v1_engine",
			body:  "INFO 10-19 12:00:00 [core.py:77] Initializing a V1 LLM engine (v0.11.0) with config: model='Qwen/Qwen3-8B', speculative_config=None, tokenizer='Qwen/Qwen3-8B', dtype=torch.bfloat16, max_seq_len=40960, tensor_parallel_size=4, pipeline_parallel_size=1, data_parallel_size=1, seed=0",
			match: true,
			want:  map[string]any{"vllm_version": "0.11.0", "vllm_engine": "V1", "tensor_parallel_size": "4"},
		},
		{
			name:  "v1_engine_core_prefix",
			body:  "(EngineCore_DP0 pid=312) INFO 10-19 12:00:00 [core.py:77] Initializing a V1 LLM engine (v0.11.0) with config: model='Qwen/Qwen3-8B', tensor_parallel_size=2, pipeline_parallel_size=1, seed=0",
			match: true,
			want:  map[string]any{"vllm_version": "0.11.0", "vllm_engine": "V1", "tensor_parallel_size": "2"},
		},
		{
			name:  "v0_engine",
			body:  "INFO 10-19 12:00:00 llm_engine.py:237] Initializing an LLM engine (v0.6.3.post1) with config: model='meta-llama/Llama-3.1-8B', tokenizer='meta-llama/Llama-3.1-8B', tensor_parallel_size=1, pipeline_parallel_size=1, seed=0",
			match: true,
			want:  map[string]any{"vllm_version": "0.6.3.post1", "tensor_parallel_size": "1"},
			// The runtime applies default_value V0; see TestVLLMEngineDefaultsOnV0Line.
			wantErr: map[string]string{"vllm_engine": "key not found in map"},
		},
		{
			name:  "missing_optional_fact",
			body:  "Initializing a V1 LLM engine (v0.11.0) with config: model='Qwen/Qwen3-8B', seed=0",
			match: true,
			want:  map[string]any{"vllm_version": "0.11.0", "vllm_engine": "V1"},
			// A missing optional fact is an evaluation error, not nil; see
			// TestConverterIndexOnMissingKey. The runtime must treat it as absent.
			wantErr: map[string]string{"tensor_parallel_size": "key not found in map"},
		},
		{
			name:     "phrase_without_config",
			body:     "WARNING 10-19 12:00:00 [core.py:90] Initializing a V1 LLM engine (v0.11.0) failed, retrying",
			match:    true,
			want:     map[string]any{"vllm_engine": "V1"},
			wantErr:  map[string]string{"vllm_version": "key not found in map", "tensor_parallel_size": "key not found in map"},
			noSeries: true,
		},
		{
			name: "other_line",
			body: "INFO 10-19 12:00:00 [api_server.py:1000] vLLM API server version 0.11.0",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lr := plog.NewLogRecord()
			lr.Body().SetStr(tc.body)
			rl := plog.NewResourceLogs()

			require.Equal(t, tc.match, ruleMatches(t, &rule, rl, lr), "match")
			if !tc.match {
				return
			}
			noSeries := false
			for _, attr := range rule.Attributes {
				got, err := evalValueErr(t, attr.valueExpression(), rl, lr)
				if wantErr, ok := tc.wantErr[attr.Key]; ok {
					assert.ErrorContains(t, err, wantErr, attr.Key)
					if !attr.Optional && attr.DefaultValue == "" {
						noSeries = true
					}
					continue
				}
				assert.NoError(t, err, attr.Key)
				assert.Equal(t, tc.want[attr.Key], got, attr.Key)
			}
			assert.Equal(t, tc.noSeries, noSeries, "a required fact without default_value failed")
		})
	}
}

// ruleMatches applies the documented matching: if body_contains is set, the
// string body must contain one of the substrings, and if conditions are set,
// one of them must be true.
func ruleMatches(t *testing.T, rule *RuleConfig, rl plog.ResourceLogs, lr plog.LogRecord) bool {
	t.Helper()
	if len(rule.BodyContains) > 0 {
		contains := false
		for _, s := range rule.BodyContains {
			contains = contains || strings.Contains(lr.Body().Str(), s)
		}
		if !contains {
			return false
		}
	}
	if len(rule.Conditions) == 0 {
		return true
	}
	for _, cond := range rule.Conditions {
		if evalCondition(t, cond, rl, lr) {
			return true
		}
	}
	return false
}

// TestVLLMEngineDefaultsOnV0Line shows that the vllm_engine expression fails
// on a V0 engine line, so the runtime applies default_value instead.
func TestVLLMEngineDefaultsOnV0Line(t *testing.T) {
	rule := loadVLLMRule(t)
	var engine *AttributeConfig
	for i := range rule.Attributes {
		if rule.Attributes[i].Key == "vllm_engine" {
			engine = &rule.Attributes[i]
		}
	}
	require.NotNil(t, engine)
	assert.Equal(t, "V0", engine.DefaultValue)
	assert.False(t, engine.Optional)

	lr := plog.NewLogRecord()
	lr.Body().SetStr("INFO 10-19 12:00:00 llm_engine.py:237] Initializing an LLM engine (v0.6.3.post1) with config: model='meta-llama/Llama-3.1-8B', tensor_parallel_size=1, seed=0")
	_, err := evalValueErr(t, engine.valueExpression(), plog.NewResourceLogs(), lr)
	assert.ErrorContains(t, err, "key not found in map")
}

// TestConverterIndexOnMissingKey pins an OTTL v0.150.0 behavior the runtime
// must handle: indexing a converter result by a missing key, as in
// ExtractPatterns(...)["v"] on a record the pattern does not match, is an
// evaluation error rather than nil, so Coalesce cannot supply a fallback.
func TestConverterIndexOnMissingKey(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.Body().SetStr("Initializing an LLM engine (v0.6.3.post1) with config: model='m'")
	for _, expr := range []string{
		`ExtractPatterns(log.body, "Initializing a (?P<v>V[01]) LLM engine")["v"]`,
		`Coalesce([ExtractPatterns(log.body, "Initializing a (?P<v>V[01]) LLM engine")["v"], "V0"])`,
	} {
		_, err := evalValueErr(t, expr, plog.NewResourceLogs(), lr)
		assert.ErrorContains(t, err, "key not found in map", expr)
	}
	// A path into a map, unlike a converter result, yields nil for a missing key.
	assert.Nil(t, evalValue(t, `log.attributes["missing"]`, plog.NewResourceLogs(), lr))
}

func evalCondition(t *testing.T, condition string, rl plog.ResourceLogs, lr plog.LogRecord) bool {
	t.Helper()
	parser, err := newLogParser(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	cond, err := parser.ParseCondition(condition)
	require.NoError(t, err, condition)
	tCtx := ottllog.NewTransformContextPtr(rl, plog.NewScopeLogs(), lr)
	defer tCtx.Close()
	got, err := cond.Eval(context.Background(), tCtx)
	require.NoError(t, err, condition)
	return got
}

func evalValue(t *testing.T, expression string, rl plog.ResourceLogs, lr plog.LogRecord) any {
	t.Helper()
	got, err := evalValueErr(t, expression, rl, lr)
	require.NoError(t, err, expression)
	return got
}

func evalValueErr(t *testing.T, expression string, rl plog.ResourceLogs, lr plog.LogRecord) (any, error) {
	t.Helper()
	parser, err := newLogParser(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	expr, err := parser.ParseValueExpression(expression)
	require.NoError(t, err, expression)
	tCtx := ottllog.NewTransformContextPtr(rl, plog.NewScopeLogs(), lr)
	defer tCtx.Close()
	return expr.Eval(context.Background(), tCtx)
}
