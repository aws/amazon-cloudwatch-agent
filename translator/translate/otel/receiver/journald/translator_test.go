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

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
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
				// Storage is optional and not configured by default
				assert.Nil(t, gotCfg.BaseConfig.StorageID)
			}
		})
	}
}