// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/registerrules"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslators(t *testing.T) {
	type want struct {
		receivers  []string
		processors []string
		exporters  []string
	}
	testCases := map[string]struct {
		input map[string]any
		want  map[string]want
	}{
		"WithContainerInsights": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"cluster_name": "TestCluster",
						},
					},
				},
			},
			want: map[string]want{
				"metrics/containerinsights": {
					receivers:  []string{"awscontainerinsightreceiver"},
					processors: []string{"batch/containerinsights", "filter/containerinsights", "awsentity/resource/containerinsights"},
					exporters:  []string{"awsemf/containerinsights"},
				},
			},
		},
		"WithEnhancedContainerInsights": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"enhanced_container_insights": true,
							"cluster_name":                "TestCluster",
						},
					},
				},
			},
			want: map[string]want{
				"metrics/containerinsights": {
					receivers:  []string{"awscontainerinsightreceiver"},
					processors: []string{"batch/containerinsights", "filter/containerinsights", "awsentity/resource/containerinsights", "metricstransform/containerinsights", "gpuattributes/containerinsights"},
					exporters:  []string{"awsemf/containerinsights"},
				},
			},
		},
		"WithContainerInsightsAndKueueMetrics": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"kueue_container_insights": true,
							"cluster_name":             "TestCluster",
						},
					},
				},
			},
			want: map[string]want{
				"metrics/containerinsights": {
					receivers:  []string{"awscontainerinsightreceiver"},
					processors: []string{"batch/containerinsights", "filter/containerinsights", "awsentity/resource/containerinsights"},
					exporters:  []string{"awsemf/containerinsights"},
				},
				"metrics/kueueContainerInsights": {
					receivers:  []string{"awscontainerinsightskueuereceiver"},
					processors: []string{"batch/kueueContainerInsights", "filter/kueueContainerInsights", "kueueattributes/kueueContainerInsights"},
					exporters:  []string{"awsemf/kueueContainerInsights"},
				},
			},
		},
		"WithEnhancedContainerInsightsAndHighFrequencyGPUMetrics": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"enhanced_container_insights":                         true,
							"accelerated_compute_metrics":                         true,
							"accelerated_compute_gpu_metrics_collection_interval": 30, // 30 seconds, less than default 60s
							"cluster_name": "TestCluster",
						},
					},
				},
			},
			want: map[string]want{
				"metrics/containerinsights": {
					receivers:  []string{"awscontainerinsightreceiver"},
					processors: []string{"batch/containerinsights", "filter/containerinsights", "groupbyattrs/containerinsights", "awsentity/resource/containerinsights", "metricstransform/containerinsights", "gpuattributes/containerinsights"},
					exporters:  []string{"awsemf/containerinsights"},
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got := NewTranslators(conf)
			if testCase.want == nil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, len(testCase.want), got.Len())
				got.Range(func(tr common.PipelineTranslator) {
					w, ok := testCase.want[tr.ID().String()]
					require.True(t, ok)
					g, err := tr.Translate(conf)
					assert.NoError(t, err)
					assert.Equal(t, w.receivers, collections.MapSlice(g.Receivers.Keys(), component.ID.String))
					assert.Equal(t, w.processors, collections.MapSlice(g.Processors.Keys(), component.ID.String))
					assert.Equal(t, w.exporters, collections.MapSlice(g.Exporters.Keys(), component.ID.String))
				})
			}
		})
	}
}
