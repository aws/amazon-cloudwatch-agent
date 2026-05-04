// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package journald

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/filestorage"
)

func TestTranslator(t *testing.T) {
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *journaldreceiver.JournaldConfig
		wantErr error
	}{
		"WithValidConfig": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"collect_list": []interface{}{
								map[string]interface{}{
									"log_group_name":    "system-logs",
									"log_stream_name":   "{instance_id}",
									"retention_in_days": 7,
									"units":             []interface{}{"systemd", "kernel", "sshd"},
									"priority":          "err",
									"matches":           []interface{}{map[string]interface{}{"_PID": "1"}},
									"filters": []interface{}{
										map[string]interface{}{
											"type":       "exclude",
											"expression": ".*debug.*",
										},
									},
								},
							},
						},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: journald.Config{
					Units:    []string{"systemd", "kernel", "sshd"},
					Priority: "err",
					Matches:  []journald.MatchConfig{{"_PID": "1"}},
				},
			},
		},
		"WithDefaultPriority": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"collect_list": []interface{}{
								map[string]interface{}{
									"log_group_name":  "default-logs",
									"log_stream_name": "{instance_id}",
								},
							},
						},
					},
				},
			},
			want: &journaldreceiver.JournaldConfig{
				InputConfig: journald.Config{
					Priority: "info",
				},
			},
		},
		"WithMissingConfig": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			filestorage.NewTranslator() // initialize StorageID before Translate
			conf := confmap.NewFromStringMap(testCase.input)
			translator := NewTranslator()
			got, err := translator.Translate(conf)

			if testCase.wantErr != nil {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)

			gotCfg, ok := got.(*journaldreceiver.JournaldConfig)
			require.True(t, ok)

			if testCase.want != nil {
				assert.Equal(t, testCase.want.InputConfig.Units, gotCfg.InputConfig.Units)
				assert.Equal(t, testCase.want.InputConfig.Priority, gotCfg.InputConfig.Priority)
				assert.Equal(t, testCase.want.InputConfig.Matches, gotCfg.InputConfig.Matches)
				// Storage is configured for cursor persistence
				assert.NotNil(t, gotCfg.BaseConfig.StorageID)
				assert.Equal(t, filestorage.StorageID, *gotCfg.BaseConfig.StorageID)
			}
		})
	}
}