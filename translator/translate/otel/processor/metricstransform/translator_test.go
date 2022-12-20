// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metricstransformprocessor

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

var nilSlice []metricstransformprocessor.Transform

func TestTranslator(t *testing.T) {
	mtpTranslator := NewTranslator()
	require.EqualValues(t, "metricstransform", mtpTranslator.Type())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *metricstransformprocessor.Config
		wantErr error
	}{
		"GenerateMetricsTransformProcessorConfig": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"net": map[string]interface{}{},
					},
				},
			},
			wantErr: &common.MissingKeyError{
				Type:    mtpTranslator.Type(),
				JsonKey: prometheusKey,
			},
		},
		"GenerateMetricsTransformProcessorConfigPrometheus": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"ecs_service_discovery": map[string]interface{}{
								"sd_target_cluster": "TestTargetCluster",
								"sd_cluster_region": "TestRegion",
								"sd_result_file":    "/result/file/path.yaml",
								"sd_frequency":      "30s",
								"docker_label": map[string]interface{}{
									"sd_job_name_label":     "ECS_PROMETHEUS_JOB_NAME_1",
									"sd_metrics_path_label": "ECS_PROMETHEUS_METRICS_PATH",
									"sd_port_label":         "ECS_PROMETHEUS_EXPORTER_PORT_SUBSET",
								},
							},
						},
					},
				},
			},
			want: &metricstransformprocessor.Config{
				Transforms: []metricstransformprocessor.Transform{
					{
						MetricIncludeFilter: metricstransformprocessor.FilterConfig{
							Include:   ".*",
							MatchType: metricstransformprocessor.RegexpMatchType,
						},
						Action: metricstransformprocessor.Update,
						Operations: []metricstransformprocessor.Operation{
							{
								Action:   metricstransformprocessor.UpdateLabel,
								NewLabel: "job",
								Label:    "prometheus_job",
							},
						},
					},
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := mtpTranslator.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*metricstransformprocessor.Config)
				require.True(t, ok)
				require.Equal(t, testCase.want.Transforms, gotCfg.Transforms)
			}
		})
	}
}
