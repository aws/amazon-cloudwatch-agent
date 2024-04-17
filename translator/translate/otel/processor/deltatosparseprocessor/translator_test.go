// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package deltatosparseprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/deltatosparseprocessor"
)

func TestTranslator(t *testing.T) {
	dtsTranslator := NewTranslator()
	require.EqualValues(t, "deltatosparse", dtsTranslator.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *deltatosparseprocessor.Config
		wantErr error
	}{
		"AcceleratedComputeFlagDisabled": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"accelerated_compute_metrics": false,
							"enhanced_container_insights": true,
						},
					},
				},
			},
			want: &deltatosparseprocessor.Config{
				Include: []string(nil),
			},
		},
		"EnhancedContainerInsightsFlagMissing": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{},
					},
				},
			},
			want: &deltatosparseprocessor.Config{
				Include: []string(nil),
			},
		},
		"AcceleratedComputeFlagEnabled": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"enhanced_container_insights": true,
						},
					},
				},
			},
			want: &deltatosparseprocessor.Config{
				Include: []string{"node_neuron_execution_errors_generic", "node_neuron_execution_errors_numerical", "node_neuron_execution_errors_transient", "node_neuron_execution_errors_model", "node_neuron_execution_errors_runtime", "node_neuron_execution_errors_hardware", "node_neuron_execution_status_completed", "node_neuron_execution_status_timed_out", "node_neuron_execution_status_completed_with_err", "node_neuron_execution_status_completed_with_num_err", "node_neuron_execution_status_incorrect_input", "node_neuron_execution_status_failed_to_queue"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := dtsTranslator.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*deltatosparseprocessor.Config)
				require.True(t, ok)
				require.Equal(t, testCase.want.Include, gotCfg.Include)
			}
		})
	}
}
