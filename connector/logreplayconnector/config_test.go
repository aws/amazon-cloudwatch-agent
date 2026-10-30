// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/mapstructure"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, &Config{
		Interval:            time.Hour,
		SourceFileAttribute: "log.file.path",
		MaxStateBytes:       4 << 20,
		MaxRecordBytes:      64 << 10,
		ErrorMode:           ottl.IgnoreError,
	}, cfg)
	assert.ErrorContains(t, xconfmap.Validate(cfg), "at least one rule must be configured")
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	storage := component.MustNewIDWithName("file_storage", "state")
	for _, tc := range []struct {
		id   component.ID
		want *Config
	}{
		{
			id: component.MustNewID(TypeStr),
			want: &Config{
				Interval:            time.Hour,
				SourceFileAttribute: "log.file.path",
				MaxStateBytes:       4 << 20,
				MaxRecordBytes:      64 << 10,
				ErrorMode:           ottl.IgnoreError,
				Rules:               []RuleConfig{{Name: "app_start", BodyContains: []string{"app started"}, Identity: []string{"k8s.pod.uid"}}},
			},
		},
		{
			id: component.MustNewIDWithName(TypeStr, "full"),
			want: &Config{
				Storage:             &storage,
				Interval:            30 * time.Minute,
				SourceFileAttribute: "log.file.path",
				MaxStateBytes:       1 << 20,
				MaxRecordBytes:      16 << 10,
				ErrorMode:           ottl.PropagateError,
				Rules: []RuleConfig{
					{
						Name:         "app_start",
						BodyContains: []string{"app started", "app restarted"},
						Conditions:   []string{`attributes["log.iostream"] == "stdout"`},
						Identity:     []string{"k8s.namespace.name", "k8s.pod.name", "k8s.container.name"},
						MaxEntries:   50,
					},
					{
						Name:       "app_ready",
						Conditions: []string{`IsMatch(body, "^ready")`},
						Identity:   []string{"k8s.pod.uid"},
					},
				},
			},
		},
	} {
		t.Run(tc.id.String(), func(t *testing.T) {
			sub, err := cm.Sub(tc.id.String())
			require.NoError(t, err)
			cfg := createDefaultConfig()
			require.NoError(t, sub.Unmarshal(cfg))
			require.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tc.want, cfg)
		})
	}
}

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		modify func(*Config)
		errs   []string
	}{
		{name: "valid", modify: func(*Config) {}},
		{
			name:   "interval_too_short",
			modify: func(c *Config) { c.Interval = 59 * time.Second },
			errs:   []string{"interval must be between 1m0s and 24h0m0s, got 59s"},
		},
		{
			name:   "interval_too_long",
			modify: func(c *Config) { c.Interval = 25 * time.Hour },
			errs:   []string{"interval must be between 1m0s and 24h0m0s, got 25h0m0s"},
		},
		{
			name:   "no_source_file_attribute",
			modify: func(c *Config) { c.SourceFileAttribute = "" },
			errs:   []string{"source_file_attribute must not be empty"},
		},
		{
			name:   "state_bytes",
			modify: func(c *Config) { c.MaxStateBytes = 1 },
			errs:   []string{"max_state_bytes must be between 65536 and 67108864, got 1"},
		},
		{
			name:   "record_bytes",
			modify: func(c *Config) { c.MaxRecordBytes = 2 << 20 },
			errs:   []string{"max_record_bytes must be between 1024 and 1048576, got 2097152"},
		},
		{
			name: "record_larger_than_state",
			modify: func(c *Config) {
				c.MaxStateBytes = minMaxStateBytes
				c.MaxRecordBytes = minMaxStateBytes + 1
			},
			errs: []string{"max_record_bytes (65537) must not be larger than max_state_bytes (65536)"},
		},
		{
			name:   "error_mode",
			modify: func(c *Config) { c.ErrorMode = "loud" },
			errs:   []string{`error_mode must be one of "propagate", "ignore", or "silent", got "loud"`},
		},
		{
			name:   "no_rules",
			modify: func(c *Config) { c.Rules = nil },
			errs:   []string{"at least one rule must be configured"},
		},
		{
			name: "rule_errors",
			modify: func(c *Config) {
				c.Rules = []RuleConfig{
					{Name: "", BodyContains: []string{""}},
					{Name: strings.Repeat("n", 256), Conditions: []string{"not ottl"}, Identity: []string{"a", "", "a"}, MaxEntries: -1},
					{Name: "dup", BodyContains: []string{"x"}, Identity: []string{"a"}, MaxEntries: maxMaxEntries + 1},
					{Name: "dup", BodyContains: []string{"x"}, Identity: []string{"a"}},
				}
			},
			errs: []string{
				"rules[0]: name must not be empty",
				"rules[0]: body_contains[0] must not be empty",
				"rules[0]: identity must list at least one resource attribute key",
				"name must be at most 255 bytes, got 256",
				"max_entries must be 0 (default 500) or between 1 and 100000, got -1",
				"conditions: ",
				"identity[1] must not be empty",
				`identity[2]: duplicate key "a", already listed at identity[0]`,
				"rules[2] (dup): max_entries must be 0 (default 500) or between 1 and 100000, got 100001",
				"rules[3] (dup): duplicate rule name, already used by rules[2]",
			},
		},
		{
			name:   "no_match",
			modify: func(c *Config) { c.Rules[0].BodyContains = nil },
			errs:   []string{"rules[0] (app_start): at least one of body_contains or conditions is required"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			tc.modify(cfg)
			err := cfg.Validate()
			if len(tc.errs) == 0 {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, want := range tc.errs {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

// TestConfigMarshal encodes a full configuration with the encoder that the
// agent uses at start to log the merged OpenTelemetry configuration. An
// error there stops the agent.
func TestConfigMarshal(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	sub, err := cm.Sub(component.MustNewIDWithName(TypeStr, "full").String())
	require.NoError(t, err)
	cfg := createDefaultConfig()
	require.NoError(t, sub.Unmarshal(cfg))
	got, err := mapstructure.Marshal(cfg)
	require.NoError(t, err)
	assert.Equal(t, "file_storage/state", got["storage"])
	assert.Equal(t, 30*time.Minute, got["interval"])
	assert.Len(t, got["rules"], 2)
}

func TestEffectiveMaxEntries(t *testing.T) {
	assert.Equal(t, defaultMaxEntries, (&RuleConfig{}).effectiveMaxEntries())
	assert.Equal(t, 3, (&RuleConfig{MaxEntries: 3}).effectiveMaxEntries())
}
