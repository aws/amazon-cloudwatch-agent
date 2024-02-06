// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metricstransformprocessor

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.Translator[component.Config] = (*translator)(nil)

func NewTranslatorWithName(name string) common.Translator[component.Config] {
	return &translator{name, metricstransformprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*metricstransformprocessor.Config)
	transformRules := []map[string]interface{}{
		{
			"include":                   "apiserver_request_total",
			"match_type":                "regexp",
			"experimental_match_labels": map[string]string{"code": "^5.*"},
			"action":                    "insert",
			"new_name":                  "apiserver_request_total_5xx",
		},
	}

	if isGpuEnabled(conf) {
		transformRules = append(transformRules, []map[string]interface{}{
			{
				"include":  "DCGM_FI_DEV_GPU_UTIL",
				"action":   "insert",
				"new_name": "pod_gpu_utilization",
			},
			{
				"include":  "DCGM_FI_DEV_GPU_UTIL",
				"action":   "insert",
				"new_name": "node_gpu_utilization",
			},
			{
				"include":  "DCGM_FI_DEV_MEM_COPY_UTIL",
				"action":   "insert",
				"new_name": "pod_gpu_utilization_memory",
			},
			{
				"include":  "DCGM_FI_DEV_MEM_COPY_UTIL",
				"action":   "insert",
				"new_name": "node_gpu_utilization_memory",
			},
			{
				"include":  "DCGM_FI_DEV_FB_USED",
				"action":   "insert",
				"new_name": "pod_gpu_memory_used",
			},
			{
				"include":  "DCGM_FI_DEV_FB_USED",
				"action":   "insert",
				"new_name": "node_gpu_memory_used",
			},
			{
				"include":          "^DCGM_FI_DEV_FB_(USED|FREE)$",
				"action":           "insert",
				"new_name":         "pod_gpu_memory_total",
				"aggregation_type": "sum",
				"match_type":       "regexp",
			},
			{
				"include":          "^DCGM_FI_DEV_FB_(USED|FREE)$",
				"action":           "insert",
				"new_name":         "node_gpu_memory_total",
				"aggregation_type": "sum",
				"match_type":       "regexp",
			},
			{
				"include":  "DCGM_FI_DEV_GPU_TEMP",
				"action":   "insert",
				"new_name": "pod_gpu_temperature",
			},
			{
				"include":  "DCGM_FI_DEV_GPU_TEMP",
				"action":   "insert",
				"new_name": "node_gpu_temperature",
			},
			{
				"include":  "DCGM_FI_DEV_POWER_USAGE",
				"action":   "insert",
				"new_name": "pod_gpu_power_draw",
			},
			{
				"include":  "DCGM_FI_DEV_POWER_USAGE",
				"action":   "insert",
				"new_name": "node_gpu_power_draw",
			},
		}...)
	}

	c := confmap.NewFromStringMap(map[string]interface{}{
		"transforms": transformRules,
	})
	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal into metricstransform config: %w", err)
	}

	return cfg, nil
}

func isGpuEnabled(conf *confmap.Conf) bool {
	return common.GetOrDefaultBool(conf, common.ConfigKey(common.LogsKey, common.MetricsCollectedKey, common.KubernetesKey, common.EnableGpuMetric), true)
}
