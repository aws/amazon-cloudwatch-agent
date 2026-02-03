//go:build linux

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	tr := NewTranslator()
	require.EqualValues(t, "journald", tr.ID().String())

	testCases := map[string]struct {
		input   map[string]interface{}
		want    *journaldreceiver.JournaldConfig
		wantErr error
	}{
		"MissingJournaldKey": {
			input: map[string]interface{}{},
			wantErr: &common.MissingKeyError{
				ID:      tr.ID(),
				JsonKey: baseKey,
			},
		},
		"EmptyJournaldConfig": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: *newDefaultInputConfig(),
			},
		},
		"WithUnits": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"units": []interface{}{"ssh", "kubelet"},
						},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: withUnits(newDefaultInputConfig(), []string{"ssh", "kubelet"}),
			},
		},
		"WithPriority": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"priority": "debug",
						},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: withPriority(newDefaultInputConfig(), "debug"),
			},
		},
		"WithStartAt": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"start_at": "beginning",
						},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: withStartAt(newDefaultInputConfig(), "beginning"),
			},
		},
		"WithDirectory": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"directory": "/var/log/journal",
						},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: withDirectory(newDefaultInputConfig(), "/var/log/journal"),
			},
		},
		"WithFiles": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"files": []interface{}{"/var/log/journal/system.journal"},
						},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: withFiles(newDefaultInputConfig(), []string{"/var/log/journal/system.journal"}),
			},
		},
		"WithIdentifiers": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"identifiers": []interface{}{"sshd", "systemd"},
						},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: withIdentifiers(newDefaultInputConfig(), []string{"sshd", "systemd"}),
			},
		},
		"WithGrep": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"grep": "error|warning",
						},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: withGrep(newDefaultInputConfig(), "error|warning"),
			},
		},
		"WithDmesg": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"dmesg": true,
						},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: withDmesg(newDefaultInputConfig(), true),
			},
		},
		"WithAll": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"all": true,
						},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: withAll(newDefaultInputConfig(), true),
			},
		},
		"WithNamespace": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"namespace": "container",
						},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: withNamespace(newDefaultInputConfig(), "container"),
			},
		},
		"WithMatches": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"matches": []interface{}{
								map[string]interface{}{
									"_SYSTEMD_UNIT": "ssh.service",
								},
							},
						},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: withMatches(newDefaultInputConfig(), []map[string]string{
					{"_SYSTEMD_UNIT": "ssh.service"},
				}),
			},
		},
		"CompleteConfig": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"units":       []interface{}{"ssh", "kubelet"},
							"priority":    "info",
							"start_at":    "end",
							"identifiers": []interface{}{"sshd"},
							"grep":        "error",
							"dmesg":       false,
							"all":         true,
							"namespace":   "default",
						},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: completeConfig(),
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tr.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*journaldreceiver.JournaldConfig)
				require.True(t, ok)
				require.Equal(t, testCase.want.InputConfig.Units, gotCfg.InputConfig.Units)
				require.Equal(t, testCase.want.InputConfig.Priority, gotCfg.InputConfig.Priority)
				require.Equal(t, testCase.want.InputConfig.StartAt, gotCfg.InputConfig.StartAt)
				require.Equal(t, testCase.want.InputConfig.Directory, gotCfg.InputConfig.Directory)
				require.Equal(t, testCase.want.InputConfig.Files, gotCfg.InputConfig.Files)
				require.Equal(t, testCase.want.InputConfig.Identifiers, gotCfg.InputConfig.Identifiers)
				require.Equal(t, testCase.want.InputConfig.Grep, gotCfg.InputConfig.Grep)
				require.Equal(t, testCase.want.InputConfig.Dmesg, gotCfg.InputConfig.Dmesg)
				require.Equal(t, testCase.want.InputConfig.All, gotCfg.InputConfig.All)
				require.Equal(t, testCase.want.InputConfig.Namespace, gotCfg.InputConfig.Namespace)
				require.Equal(t, testCase.want.InputConfig.Matches, gotCfg.InputConfig.Matches)
			}
		})
	}
}

func TestTranslatorWithName(t *testing.T) {
	tr := NewTranslatorWithName("custom")
	require.EqualValues(t, "journald/custom", tr.ID().String())
}

// Helper functions to build expected configs
func newDefaultInputConfig() *journaldreceiver.JournaldConfig {
	factory := journaldreceiver.NewFactory()
	return factory.CreateDefaultConfig().(*journaldreceiver.JournaldConfig)
}

func withUnits(cfg *journaldreceiver.JournaldConfig, units []string) journaldreceiver.JournaldConfig {
	result := *cfg
	result.InputConfig.Units = units
	return result
}

func withPriority(cfg *journaldreceiver.JournaldConfig, priority string) journaldreceiver.JournaldConfig {
	result := *cfg
	result.InputConfig.Priority = priority
	return result
}

func withStartAt(cfg *journaldreceiver.JournaldConfig, startAt string) journaldreceiver.JournaldConfig {
	result := *cfg
	result.InputConfig.StartAt = startAt
	return result
}

func withDirectory(cfg *journaldreceiver.JournaldConfig, directory string) journaldreceiver.JournaldConfig {
	result := *cfg
	result.InputConfig.Directory = &directory
	return result
}

func withFiles(cfg *journaldreceiver.JournaldConfig, files []string) journaldreceiver.JournaldConfig {
	result := *cfg
	result.InputConfig.Files = files
	return result
}

func withIdentifiers(cfg *journaldreceiver.JournaldConfig, identifiers []string) journaldreceiver.JournaldConfig {
	result := *cfg
	result.InputConfig.Identifiers = identifiers
	return result
}

func withGrep(cfg *journaldreceiver.JournaldConfig, grep string) journaldreceiver.JournaldConfig {
	result := *cfg
	result.InputConfig.Grep = grep
	return result
}

func withDmesg(cfg *journaldreceiver.JournaldConfig, dmesg bool) journaldreceiver.JournaldConfig {
	result := *cfg
	result.InputConfig.Dmesg = dmesg
	return result
}

func withAll(cfg *journaldreceiver.JournaldConfig, all bool) journaldreceiver.JournaldConfig {
	result := *cfg
	result.InputConfig.All = all
	return result
}

func withNamespace(cfg *journaldreceiver.JournaldConfig, namespace string) journaldreceiver.JournaldConfig {
	result := *cfg
	result.InputConfig.Namespace = namespace
	return result
}

func withMatches(cfg *journaldreceiver.JournaldConfig, matches []map[string]string) journaldreceiver.JournaldConfig {
	result := *cfg
	result.InputConfig.Matches = matches
	return result
}

func completeConfig() journaldreceiver.JournaldConfig {
	cfg := newDefaultInputConfig()
	cfg.InputConfig.Units = []string{"ssh", "kubelet"}
	cfg.InputConfig.Priority = "info"
	cfg.InputConfig.StartAt = "end"
	cfg.InputConfig.Identifiers = []string{"sshd"}
	cfg.InputConfig.Grep = "error"
	cfg.InputConfig.Dmesg = false
	cfg.InputConfig.All = true
	cfg.InputConfig.Namespace = "default"
	return *cfg
}
