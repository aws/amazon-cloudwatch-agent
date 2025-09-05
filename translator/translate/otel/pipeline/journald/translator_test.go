// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package journald

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
)

func TestTranslator(t *testing.T) {
	testCases := map[string]struct {
		input   map[string]interface{}
		wantErr bool
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
			wantErr: false,
		},
		"WithMissingConfig": {
			input:   map[string]interface{}{},
			wantErr: true,
		},
		"WithEmptyCollectList": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"collect_list": []interface{}{},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			translator := NewTranslator()
			
			// Verify ID
			expectedID := pipeline.NewIDWithName(pipeline.SignalLogs, pipelineName)
			assert.Equal(t, expectedID, translator.ID())

			got, err := translator.Translate(conf)

			if testCase.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)

			// Verify components are created
			assert.True(t, got.Receivers.Len() > 0, "Should have at least one receiver")
			assert.True(t, got.Processors.Len() > 0, "Should have at least one processor")
			assert.True(t, got.Exporters.Len() > 0, "Should have at least one exporter")
			assert.True(t, got.Extensions.Len() > 0, "Should have at least one extension")
		})
	}
}

func TestNewTranslators(t *testing.T) {
	testCases := map[string]struct {
		input       map[string]interface{}
		expectCount int
	}{
		"WithJournaldConfig": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"collect_list": []interface{}{
								map[string]interface{}{
									"log_group_name": "test-logs",
								},
							},
						},
					},
				},
			},
			expectCount: 1,
		},
		"WithoutJournaldConfig": {
			input:       map[string]interface{}{},
			expectCount: 0,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			translators := NewTranslators(conf)
			assert.Equal(t, testCase.expectCount, translators.Len())
		})
	}
}