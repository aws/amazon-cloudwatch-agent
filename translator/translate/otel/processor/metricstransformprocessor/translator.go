// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metricstransformprocessor

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/awscontainerinsight"
)

const gpuLogSuffix = "GPU"

var metricDuplicateTypes = []string{
	containerinsightscommon.TypeGpuContainer,
	containerinsightscommon.TypeGpuPod,
	containerinsightscommon.TypeGpuNode,
}

var renameMapForDcgm = map[string]string{
	"DCGM_FI_DEV_GPU_UTIL":        containerinsightscommon.GpuUtilization,
	"DCGM_FI_DEV_FB_USED_PERCENT": containerinsightscommon.GpuMemUtilization,
	"DCGM_FI_DEV_FB_USED":         containerinsightscommon.GpuMemUsed,
	"DCGM_FI_DEV_FB_TOTAL":        containerinsightscommon.GpuMemTotal,
	"DCGM_FI_DEV_GPU_TEMP":        containerinsightscommon.GpuTemperature,
	"DCGM_FI_DEV_POWER_USAGE":     containerinsightscommon.GpuPowerDraw,
}

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

	if awscontainerinsight.AcceleratedComputeMetricsEnabled(conf) {
		// appends DCGM metric transform rules for each metric type (container/pod/node) with following format:
		// {
		//		"include":  "DCGM_FI_DEV_GPU_UTIL",
		//		"action":   "insert",
		//		"new_name": "container_gpu_utilization",
		//		"operations": [
		//     		{
		//				"action":   "add_label",
		//				"new_label": "Type",
		//				"new_value": "ContainerGPU",
		//			},
		//			<additional operations>...
		//      ]
		//	},
		for old, new := range renameMapForDcgm {
			var operations []map[string]interface{}
			// convert decimals to percent
			if new == containerinsightscommon.GpuMemUtilization {
				operations = append(operations, map[string]interface{}{
					"action":             "experimental_scale_value",
					"experimental_scale": 100,
				})
			} else if new == containerinsightscommon.GpuMemTotal || new == containerinsightscommon.GpuMemUsed {
				operations = append(operations, map[string]interface{}{
					"action":             "experimental_scale_value",
					"experimental_scale": 1024 * 1024,
				})
			}
			for _, t := range metricDuplicateTypes {
				transformRules = append(transformRules, map[string]interface{}{
					"include":  old,
					"action":   "insert",
					"new_name": containerinsightscommon.MetricName(t, new),
					"operations": append([]map[string]interface{}{
						{
							"action":    "add_label",
							"new_label": containerinsightscommon.MetricType,
							"new_value": t,
						},
					}, operations...),
				})
			}
		}
	}

	c := confmap.NewFromStringMap(map[string]interface{}{
		"transforms": transformRules,
	})
	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal into metricstransform config: %w", err)
	}

	return cfg, nil
}
