// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package resourceprocessor

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	rpTranslator := NewTranslator()
	require.EqualValues(t, "resource", rpTranslator.Type())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    []map[string]interface{} // Can't construct & use resourceprocessor.Config as it uses internal only types
		wantErr error
	}{
		"GenerateResourceProcessorConfig": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"net": map[string]interface{}{},
					},
				},
			},
			wantErr: &common.MissingKeyError{
				Type:    rpTranslator.Type(),
				JsonKey: prometheusKey,
			},
		},
		"GenerateResourceProcessorConfigPrometheus": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"prometheus": map[string]interface{}{},
					},
				},
			},
			want: []map[string]interface{}{
				{
					"key":            "job",
					"value":          nil,
					"pattern":        "",
					"from_attribute": "service.name",
					"from_context":   "",
					"converted_type": "",
					"action":         "upsert",
				},
				{
					"key":            "ServiceName",
					"value":          nil,
					"pattern":        "",
					"from_attribute": "service.name",
					"from_context":   "",
					"converted_type": "",
					"action":         "upsert",
				},
				{
					"key":            "service.name",
					"value":          nil,
					"pattern":        "",
					"from_attribute": "",
					"from_context":   "",
					"converted_type": "",
					"action":         "delete",
				},
				{
					"key":            "instance",
					"value":          nil,
					"pattern":        "",
					"from_attribute": "service.instance.id",
					"from_context":   "",
					"converted_type": "",
					"action":         "upsert",
				},
				{
					"key":            "service.instance.id",
					"value":          nil,
					"pattern":        "",
					"from_attribute": "",
					"from_context":   "",
					"converted_type": "",
					"action":         "delete",
				},
				{
					"key":            "net.host.port",
					"value":          nil,
					"pattern":        "",
					"from_attribute": "",
					"from_context":   "",
					"converted_type": "",
					"action":         "delete",
				},
				{
					"key":            "http.scheme",
					"value":          nil,
					"pattern":        "",
					"from_attribute": "",
					"from_context":   "",
					"converted_type": "",
					"action":         "delete",
				},
				{
					"key":            "Version",
					"value":          1,
					"pattern":        "",
					"from_attribute": "",
					"from_context":   "",
					"converted_type": "",
					"action":         "insert",
				},
				{
					"key":            "receiver",
					"value":          "prometheus",
					"pattern":        "",
					"from_attribute": "",
					"from_context":   "",
					"converted_type": "",
					"action":         "insert",
				},
			},
		},
		"GenerateResourceProcessorConfigPrometheusWithEcsSD": {
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
			want: []map[string]interface{}{
				{
					"key":            "service.name",
					"value":          nil,
					"pattern":        "",
					"from_attribute": "",
					"from_context":   "",
					"converted_type": "",
					"action":         "delete",
				},
				{
					"key":            "instance",
					"value":          nil,
					"pattern":        "",
					"from_attribute": "service.instance.id",
					"from_context":   "",
					"converted_type": "",
					"action":         "upsert",
				},
				{
					"key":            "service.instance.id",
					"value":          nil,
					"pattern":        "",
					"from_attribute": "",
					"from_context":   "",
					"converted_type": "",
					"action":         "delete",
				},
				{
					"key":            "net.host.port",
					"value":          nil,
					"pattern":        "",
					"from_attribute": "",
					"from_context":   "",
					"converted_type": "",
					"action":         "delete",
				},
				{
					"key":            "http.scheme",
					"value":          nil,
					"pattern":        "",
					"from_attribute": "",
					"from_context":   "",
					"converted_type": "",
					"action":         "delete",
				},
				{
					"key":            "Version",
					"value":          1,
					"pattern":        "",
					"from_attribute": "",
					"from_context":   "",
					"converted_type": "",
					"action":         "insert",
				},
				{
					"key":            "receiver",
					"value":          "prometheus",
					"pattern":        "",
					"from_attribute": "",
					"from_context":   "",
					"converted_type": "",
					"action":         "insert",
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := rpTranslator.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*resourceprocessor.Config)
				require.True(t, ok)
				require.Equal(t, len(testCase.want), len(gotCfg.AttributesActions))
				for i := range gotCfg.AttributesActions {
					require.Equal(t, testCase.want[i]["key"], gotCfg.AttributesActions[i].Key)
					require.Equal(t, testCase.want[i]["pattern"], gotCfg.AttributesActions[i].RegexPattern)
					require.Equal(t, testCase.want[i]["from_attribute"], gotCfg.AttributesActions[i].FromAttribute)
					require.Equal(t, testCase.want[i]["from_context"], gotCfg.AttributesActions[i].FromContext)
					require.Equal(t, testCase.want[i]["converted_type"], gotCfg.AttributesActions[i].ConvertedType)
					require.Equal(t, testCase.want[i]["action"], string(gotCfg.AttributesActions[i].Action))
					switch gotCfg.AttributesActions[i].Value.(type) {
					case string:
						require.Equal(t, testCase.want[i]["value"], gotCfg.AttributesActions[i].Value.(string))
					case int:
						require.Equal(t, testCase.want[i]["value"], gotCfg.AttributesActions[i].Value.(int))
					case nil:
						require.Equal(t, testCase.want[i]["value"], gotCfg.AttributesActions[i].Value)
					default:
						require.Failf(t, "Unexpected value type for value field in resource processor action",
							"Value %v was of type %T", gotCfg.AttributesActions[i].Value, gotCfg.AttributesActions[i].Value)
					}
				}
			}
		})
	}
}
