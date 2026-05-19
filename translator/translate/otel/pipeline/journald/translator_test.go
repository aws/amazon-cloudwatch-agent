// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package journald

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	testCases := map[string]struct {
		input      map[string]interface{}
		translator common.PipelineTranslator
		wantErr    bool
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
			translator: NewTranslator(common.WithIndex(0)),
			wantErr:    false,
		},
		"WithMissingConfig": {
			input:      map[string]interface{}{},
			translator: NewTranslator(common.WithIndex(0)),
			wantErr:    true,
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
			translator: NewTranslator(common.WithIndex(0)),
			wantErr:    true,
		},
		"WithMultipleEntries": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"collect_list": []interface{}{
								map[string]interface{}{
									"log_group_name": "first-logs",
									"units":          []interface{}{"systemd"},
								},
								map[string]interface{}{
									"log_group_name": "second-logs",
									"units":          []interface{}{"kernel", "sshd"},
									"priority":       "warning",
								},
							},
						},
					},
				},
			},
			translator: NewTranslator(common.WithIndex(1)),
			wantErr:    false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			tt := testCase.translator

			got, err := tt.Translate(conf)

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

func TestTranslatorID(t *testing.T) {
	// Without index
	tt := NewTranslator()
	assert.Equal(t, pipeline.NewIDWithName(pipeline.SignalLogs, pipelineName), tt.ID())

	// With index
	tt = NewTranslator(common.WithIndex(0))
	assert.Equal(t, pipeline.NewIDWithName(pipeline.SignalLogs, pipelineName+"/0"), tt.ID())

	tt = NewTranslator(common.WithIndex(2))
	assert.Equal(t, pipeline.NewIDWithName(pipeline.SignalLogs, pipelineName+"/2"), tt.ID())
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
		"WithMultipleCollectListEntries": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"logs_collected": map[string]interface{}{
						"journald": map[string]interface{}{
							"collect_list": []interface{}{
								map[string]interface{}{"log_group_name": "logs-1"},
								map[string]interface{}{"log_group_name": "logs-2"},
								map[string]interface{}{"log_group_name": "logs-3"},
							},
						},
					},
				},
			},
			expectCount: 3,
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
