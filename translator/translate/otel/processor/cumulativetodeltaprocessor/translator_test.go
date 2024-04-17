// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cumulativetodeltaprocessor

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslator(t *testing.T) {
	cdpTranslator := NewTranslator()
	require.EqualValues(t, "cumulativetodelta", cdpTranslator.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *cumulativetodeltaprocessor.Config
		wantErr error
	}{
		"GenerateDeltaProcessorConfigWithCPU": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"accelerated_compute_metrics": true,
							"enhanced_container_insights": true,
						},
					},
				},
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"cpu": map[string]interface{}{},
					},
				},
			},
			want: &cumulativetodeltaprocessor.Config{
				Include: cumulativetodeltaprocessor.MatchMetrics{Metrics: []string{
					"node_neuron_execution_*",
					"container_neurondevice_hw_ecc_events_*",
					"pod_neurondevice_hw_ecc_events_*",
					"node_neurondevice_hw_ecc_events_*"}},
				Exclude: cumulativetodeltaprocessor.MatchMetrics{},
			},
		},
		"GenerateDeltaProcessorConfigWithNet": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"accelerated_compute_metrics": true,
							"enhanced_container_insights": true,
						},
					},
				},
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"net": map[string]interface{}{},
					},
				},
			},
			want: &cumulativetodeltaprocessor.Config{
				Include: cumulativetodeltaprocessor.MatchMetrics{Metrics: []string{
					"node_neuron_execution_*",
					"container_neurondevice_hw_ecc_events_*",
					"pod_neurondevice_hw_ecc_events_*",
					"node_neurondevice_hw_ecc_events_*"}},
				Exclude: cumulativetodeltaprocessor.MatchMetrics{},
			},
		},
		"GenerateDeltaProcessorConfigWithDiskIO": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"accelerated_compute_metrics": true,
							"enhanced_container_insights": true,
						},
					},
				},
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"diskio": map[string]interface{}{},
					},
				},
			},
			want: &cumulativetodeltaprocessor.Config{
				Include: cumulativetodeltaprocessor.MatchMetrics{Metrics: []string{
					"node_neuron_execution_*",
					"container_neurondevice_hw_ecc_events_*",
					"pod_neurondevice_hw_ecc_events_*",
					"node_neurondevice_hw_ecc_events_*"}},
				Exclude: cumulativetodeltaprocessor.MatchMetrics{Metrics: []string{"iops_in_progress", "diskio_iops_in_progress"}},
			},
		},
		"GenerateWithoutEnhancedContainerInsights": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"accelerated_compute_metrics": true,
						},
					},
				},
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"diskio": map[string]interface{}{},
					},
				},
			},
			want: &cumulativetodeltaprocessor.Config{
				Include: cumulativetodeltaprocessor.MatchMetrics{},
				Exclude: cumulativetodeltaprocessor.MatchMetrics{Metrics: []string{"iops_in_progress", "diskio_iops_in_progress"}},
			},
		},
		"GenerateWithAcceleratedComputeFalse": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"enhanced_container_insights": true,
							"accelerated_compute_metrics": false,
						},
					},
				},
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"diskio": map[string]interface{}{},
					},
				},
			},
			want: &cumulativetodeltaprocessor.Config{
				Include: cumulativetodeltaprocessor.MatchMetrics{},
				Exclude: cumulativetodeltaprocessor.MatchMetrics{Metrics: []string{"iops_in_progress", "diskio_iops_in_progress"}},
			},
		},
		"GenerateWithoutAcceleratedComputeFlag": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"enhanced_container_insights": true,
						},
					},
				},
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"diskio": map[string]interface{}{},
					},
				},
			},
			want: &cumulativetodeltaprocessor.Config{
				Include: cumulativetodeltaprocessor.MatchMetrics{Metrics: []string{
					"node_neuron_execution_*",
					"container_neurondevice_hw_ecc_events_*",
					"pod_neurondevice_hw_ecc_events_*",
					"node_neurondevice_hw_ecc_events_*"}},
				Exclude: cumulativetodeltaprocessor.MatchMetrics{Metrics: []string{"iops_in_progress", "diskio_iops_in_progress"}},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := cdpTranslator.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*cumulativetodeltaprocessor.Config)
				require.True(t, ok)
				require.Equal(t, testCase.want.Include.Metrics, gotCfg.Include.Metrics)
				require.Equal(t, testCase.want.Exclude.Metrics, gotCfg.Exclude.Metrics)
			}
		})
	}
}
