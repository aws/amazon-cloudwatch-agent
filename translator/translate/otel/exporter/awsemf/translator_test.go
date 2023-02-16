// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	legacytranslator "github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

var nilSlice []string
var nilMetricDescriptorsSlice []awsemfexporter.MetricDescriptor

func TestTranslator(t *testing.T) {
	tt := NewTranslator()
	require.EqualValues(t, "awsemf", tt.Type())
	testCases := map[string]struct {
		env     map[string]string
		input   map[string]interface{}
		want    map[string]interface{} // Can't construct & use awsemfexporter.Config as it uses internal only types
		wantErr error
	}{
		"GenerateAwsEmfExporterConfigEcs": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"ecs": map[string]interface{}{},
					},
				},
			},
			want: map[string]interface{}{
				"namespace":                              "ECS/ContainerInsights",
				"log_group_name":                         "/aws/ecs/containerinsights/{ClusterName}/performance",
				"log_stream_name":                        "instanceTelemetry/{ContainerInstanceId}",
				"dimension_rollup_option":                "NoDimensionRollup",
				"parse_json_encoded_attr_values":         []string{"Sources"},
				"output_destination":                     "cloudwatch",
				"eks_fargate_container_insights_enabled": false,
				"resource_to_telemetry_conversion": resourcetotelemetry.Settings{
					Enabled: true,
				},
				"metric_declarations": []*awsemfexporter.MetricDeclaration{
					{
						Dimensions: [][]string{{"ContainerInstanceId", "InstanceId", "ClusterName"}},
						MetricNameSelectors: []string{"instance_cpu_reserved_capacity", "instance_cpu_utilization",
							"instance_filesystem_utilization", "instance_memory_reserved_capacity",
							"instance_memory_utilization", "instance_network_total_bytes", "instance_number_of_running_tasks"},
					},
					{
						Dimensions: [][]string{{"ClusterName"}},
						MetricNameSelectors: []string{"instance_cpu_limit", "instance_cpu_reserved_capacity",
							"instance_cpu_usage_total", "instance_cpu_utilization", "instance_filesystem_utilization",
							"instance_memory_limit", "instance_memory_reserved_capacity", "instance_memory_utilization",
							"instance_memory_working_set", "instance_network_total_bytes", "instance_number_of_running_tasks"},
					},
				},
				"metric_descriptors": nilMetricDescriptorsSlice,
			},
		},
		"GenerateAwsEmfExporterConfigPrometheus": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"log_group_name":  "/test/log/group",
							"log_stream_name": "{ServiceName}",
							"emf_processor": map[string]interface{}{
								"metric_declaration": []interface{}{
									map[string]interface{}{
										"source_labels":    []string{"Service", "Namespace"},
										"label_matcher":    "(.*node-exporter.*|.*kube-dns.*);kube-system$",
										"dimensions":       [][]string{{"Service", "Namespace"}},
										"metric_selectors": []string{"^coredns_dns_request_type_count_total$"},
									},
								},
								"metric_unit": map[string]interface{}{
									"jvm_gc_collection_seconds_sum": "Milliseconds",
								},
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"namespace":                              "CWAgent/Prometheus",
				"log_group_name":                         "/test/log/group",
				"log_stream_name":                        "{ServiceName}",
				"dimension_rollup_option":                "NoDimensionRollup",
				"parse_json_encoded_attr_values":         nilSlice,
				"output_destination":                     "cloudwatch",
				"eks_fargate_container_insights_enabled": false,
				"resource_to_telemetry_conversion": resourcetotelemetry.Settings{
					Enabled: true,
				},
				"metric_declarations": []*awsemfexporter.MetricDeclaration{
					{
						Dimensions:          [][]string{{"Service", "Namespace"}},
						MetricNameSelectors: []string{"^coredns_dns_request_type_count_total$"},
						LabelMatchers: []*awsemfexporter.LabelMatcher{
							{
								LabelNames: []string{"Service", "Namespace"},
								Regex:      "(.*node-exporter.*|.*kube-dns.*);kube-system$",
							},
						},
					},
				},
				"metric_descriptors": []awsemfexporter.MetricDescriptor{
					{
						MetricName: "jvm_gc_collection_seconds_sum",
						Unit:       "Milliseconds",
					},
				},
			},
		},
		"GenerateAwsEmfExporterConfigPrometheusNoDeclarations": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"log_group_name":  "/test/log/group",
							"log_stream_name": "{ServiceName}",
							"emf_processor": map[string]interface{}{
								"metric_unit": map[string]interface{}{
									"jvm_gc_collection_seconds_sum": "Milliseconds",
								},
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"namespace":                              "CWAgent/Prometheus",
				"log_group_name":                         "/test/log/group",
				"log_stream_name":                        "{ServiceName}",
				"dimension_rollup_option":                "NoDimensionRollup",
				"parse_json_encoded_attr_values":         nilSlice,
				"output_destination":                     "cloudwatch",
				"eks_fargate_container_insights_enabled": false,
				"resource_to_telemetry_conversion": resourcetotelemetry.Settings{
					Enabled: true,
				},
				"metric_declarations": []*awsemfexporter.MetricDeclaration{
					{
						MetricNameSelectors: []string{"$^"},
					},
				},
				"metric_descriptors": []awsemfexporter.MetricDescriptor{
					{
						MetricName: "jvm_gc_collection_seconds_sum",
						Unit:       "Milliseconds",
					},
				},
			},
		},
		"GenerateAwsEmfExporterConfigPrometheusNoEmfProcessor": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"log_group_name":  "/test/log/group",
							"log_stream_name": "{ServiceName}",
						},
					},
				},
			},
			want: map[string]interface{}{
				"namespace":                              "",
				"log_group_name":                         "/test/log/group",
				"log_stream_name":                        "{ServiceName}",
				"dimension_rollup_option":                "NoDimensionRollup",
				"parse_json_encoded_attr_values":         nilSlice,
				"output_destination":                     "cloudwatch",
				"eks_fargate_container_insights_enabled": false,
				"resource_to_telemetry_conversion": resourcetotelemetry.Settings{
					Enabled: true,
				},
				"metric_declarations": []*awsemfexporter.MetricDeclaration{
					{
						MetricNameSelectors: []string{"$^"},
					},
				},
				"metric_descriptors": nilMetricDescriptorsSlice,
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			require.Truef(t, legacytranslator.IsTranslateSuccess(), "Error in legacy translation rules: %v", legacytranslator.ErrorMessages)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*awsemfexporter.Config)
				require.True(t, ok)
				require.Equal(t, testCase.want["namespace"], gotCfg.Namespace)
				require.Equal(t, testCase.want["log_group_name"], gotCfg.LogGroupName)
				require.Equal(t, testCase.want["log_stream_name"], gotCfg.LogStreamName)
				require.Equal(t, testCase.want["dimension_rollup_option"], gotCfg.DimensionRollupOption)
				require.Equal(t, testCase.want["parse_json_encoded_attr_values"], gotCfg.ParseJSONEncodedAttributeValues)
				require.Equal(t, testCase.want["output_destination"], gotCfg.OutputDestination)
				require.Equal(t, testCase.want["eks_fargate_container_insights_enabled"], gotCfg.EKSFargateContainerInsightsEnabled)
				require.Equal(t, testCase.want["resource_to_telemetry_conversion"], gotCfg.ResourceToTelemetrySettings)
				require.Equal(t, testCase.want["metric_declarations"], gotCfg.MetricDeclarations)
				require.Equal(t, testCase.want["metric_descriptors"], gotCfg.MetricDescriptors)
			}
		})
	}
}
