// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslator(t *testing.T) {
	translator := NewTranslator()
	require.EqualValues(t, "journald", translator.ID().Type().String())
	require.EqualValues(t, "", translator.ID().Name())

	translatorWithName := NewTranslatorWithName("test")
	require.EqualValues(t, "journald", translatorWithName.ID().Type().String())
	require.EqualValues(t, "test", translatorWithName.ID().Name())
}

func TestTranslate(t *testing.T) {
	translator := NewTranslator()

	testCases := map[string]struct {
		input map[string]interface{}
		want  *journaldreceiver.JournaldConfig
	}{
		"WithMinimalConfig": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{},
					},
				},
			},
			want: func() *journaldreceiver.JournaldConfig {
				cfg := &journaldreceiver.JournaldConfig{}
				cfg.InputConfig.Priority = "info"
				cfg.InputConfig.StartAt = "end"
				return cfg
			}(),
		},
		"WithAllFields": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"directory":   "/var/log/journal",
							"files":       []interface{}{"system.journal"},
							"units":       []interface{}{"sshd.service", "nginx.service"},
							"identifiers": []interface{}{"kernel"},
							"priority":    "info",
							"grep":        "error",
							"matches": []interface{}{
								map[string]interface{}{"_SYSTEMD_UNIT": "sshd.service"},
								map[string]interface{}{"PRIORITY": "6"},
							},
							"dmesg":    true,
							"all":      false,
							"start_at": "end",
						},
					},
				},
			},
			want: func() *journaldreceiver.JournaldConfig {
				cfg := &journaldreceiver.JournaldConfig{}
				directory := "/var/log/journal"
				cfg.InputConfig.Directory = &directory
				cfg.InputConfig.Files = []string{"system.journal"}
				cfg.InputConfig.Units = []string{"sshd.service", "nginx.service"}
				cfg.InputConfig.Identifiers = []string{"kernel"}
				cfg.InputConfig.Priority = "info"
				cfg.InputConfig.Grep = "error"
				cfg.InputConfig.Matches = []journald.MatchConfig{
					{"_SYSTEMD_UNIT": "sshd.service"},
					{"PRIORITY": "6"},
				}
				cfg.InputConfig.Dmesg = true
				cfg.InputConfig.All = false
				cfg.InputConfig.StartAt = "end"
				return cfg
			}(),
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := translator.Translate(conf)
			require.NoError(t, err)
			require.NotNil(t, got)
			gotCfg, ok := got.(*journaldreceiver.JournaldConfig)
			require.True(t, ok)

			if testCase.want.InputConfig.Directory != nil {
				assert.Equal(t, *testCase.want.InputConfig.Directory, *gotCfg.InputConfig.Directory)
			}
			assert.Equal(t, testCase.want.InputConfig.Files, gotCfg.InputConfig.Files)
			assert.Equal(t, testCase.want.InputConfig.Units, gotCfg.InputConfig.Units)
			assert.Equal(t, testCase.want.InputConfig.Identifiers, gotCfg.InputConfig.Identifiers)
			assert.Equal(t, testCase.want.InputConfig.Priority, gotCfg.InputConfig.Priority)
			assert.Equal(t, testCase.want.InputConfig.Grep, gotCfg.InputConfig.Grep)
			assert.Equal(t, testCase.want.InputConfig.Matches, gotCfg.InputConfig.Matches)
			assert.Equal(t, testCase.want.InputConfig.Dmesg, gotCfg.InputConfig.Dmesg)
			assert.Equal(t, testCase.want.InputConfig.All, gotCfg.InputConfig.All)
			assert.Equal(t, testCase.want.InputConfig.StartAt, gotCfg.InputConfig.StartAt)
		})
	}
}

func TestTranslateMissingKey(t *testing.T) {
	translator := NewTranslator()
	conf := confmap.NewFromStringMap(map[string]interface{}{})
	_, err := translator.Translate(conf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing key")
}