// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metricstransformprocessor

import (
	_ "embed"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor"

	"github.com/aws/amazon-cloudwatch-agent/internal/constants"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/receiver/awscontainerinsight"
)

//go:embed metricstransform_jmx_config.yaml
var metricTransformJmxConfig string

//go:embed appsignals_runtime_config.yaml
var appSignalsRuntimeConfig string

var metricDuplicateTypes = []string{
	constants.TypeGpuContainer,
	constants.TypeGpuPod,
	constants.TypeGpuNode,
}

var renameMapForDcgm = map[string]string{
	"DCGM_FI_DEV_GPU_UTIL":        constants.GpuUtilization,
	"DCGM_FI_DEV_FB_USED_PERCENT": constants.GpuMemUtilization,
	"DCGM_FI_DEV_FB_USED":         constants.GpuMemUsed,
	"DCGM_FI_DEV_FB_TOTAL":        constants.GpuMemTotal,
	"DCGM_FI_DEV_GPU_TEMP":        constants.GpuTemperature,
	"DCGM_FI_DEV_POWER_USAGE":     constants.GpuPowerDraw,
}

var renameMapForNeuronMonitor = map[string]string{
	"execution_errors_total":                          constants.NeuronExecutionErrors,
	"execution_status_total":                          constants.NeuronExecutionStatus,
	"neuron_runtime_memory_used_bytes":                constants.NeuronRuntimeMemoryUsage,
	"neuroncore_memory_usage_constants":               constants.NeuronCoreMemoryUtilizationConstants,
	"neuroncore_memory_usage_model_code":              constants.NeuronCoreMemoryUtilizationModelCode,
	"neuroncore_memory_usage_model_shared_scratchpad": constants.NeuronCoreMemoryUtilizationSharedScratchpad,
	"neuroncore_memory_usage_runtime_memory":          constants.NeuronCoreMemoryUtilizationRuntimeMemory,
	"neuroncore_memory_usage_tensors":                 constants.NeuronCoreMemoryUtilizationTensors,
	"neuroncore_utilization_ratio":                    constants.NeuronCoreUtilization,
	"instance_info":                                   constants.NeuronInstanceInfo,
	"neuron_hardware":                                 constants.NeuronHardware,
	"hardware_ecc_events_total":                       constants.NeuronDeviceHardwareEccEvents,
	"execution_latency_seconds":                       constants.NeuronExecutionLatency,
}

var renameMapForNvme = map[string]string{
	"aws_ebs_csi_read_ops_total":                  constants.NvmeReadOpsTotal,
	"aws_ebs_csi_write_ops_total":                 constants.NvmeWriteOpsTotal,
	"aws_ebs_csi_read_bytes_total":                constants.NvmeReadBytesTotal,
	"aws_ebs_csi_write_bytes_total":               constants.NvmeWriteBytesTotal,
	"aws_ebs_csi_read_seconds_total":              constants.NvmeReadTime,
	"aws_ebs_csi_write_seconds_total":             constants.NvmeWriteTime,
	"aws_ebs_csi_exceeded_iops_seconds_total":     constants.NvmeExceededIOPSTime,
	"aws_ebs_csi_exceeded_tp_seconds_total":       constants.NvmeExceededTPTime,
	"aws_ebs_csi_ec2_exceeded_iops_seconds_total": constants.NvmeExceededEC2IOPSTime,
	"aws_ebs_csi_ec2_exceeded_tp_seconds_total":   constants.NvmeExceededEC2TPTime,
	"aws_ebs_csi_volume_queue_length":             constants.NvmeVolumeQueueLength,
}

type translator struct {
	name    string
	factory processor.Factory
}

var _ common.ComponentTranslator = (*translator)(nil)

func NewTranslatorWithName(name string) common.ComponentTranslator {
	return &translator{name, metricstransformprocessor.NewFactory()}
}

func (t *translator) ID() component.ID {
	return component.NewIDWithName(t.factory.Type(), t.name)
}

func (t *translator) Translate(conf *confmap.Conf) (component.Config, error) {
	cfg := t.factory.CreateDefaultConfig().(*metricstransformprocessor.Config)
	if t.name == common.PipelineNameContainerInsightsJmx {
		return common.GetYamlFileToYamlConfig(cfg, metricTransformJmxConfig)
	} else if t.name == common.AppSignals {
		return common.GetYamlFileToYamlConfig(cfg, appSignalsRuntimeConfig)
	}

	var transformRules []map[string]interface{}
	if t.name == common.PipelineNameContainerInsights {
		transformRules = []map[string]interface{}{
			{
				"include":                   "apiserver_request_total",
				"match_type":                "regexp",
				"experimental_match_labels": map[string]string{"code": "^5.*"},
				"action":                    "insert",
				"new_name":                  "apiserver_request_total_5xx",
			},
		}

		if awscontainerinsight.EnhancedContainerInsightsEnabled(conf) {
			for oldNvmeMetric, newNvmeMetric := range renameMapForNvme {
				transformRules = append(transformRules, map[string]interface{}{
					"include":  oldNvmeMetric,
					"action":   "update",
					"new_name": metricName(constants.TypeNode, newNvmeMetric),
					"operations": []map[string]interface{}{{
						"action":    "add_label",
						"new_label": constants.MetricType,
						"new_value": constants.TypeNodeEBS,
					}},
				})
			}

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
				if new == constants.GpuMemUtilization {
					operations = append(operations, map[string]interface{}{
						"action":             "experimental_scale_value",
						"experimental_scale": 100,
					})
				} else if new == constants.GpuMemTotal || new == constants.GpuMemUsed {
					operations = append(operations, map[string]interface{}{
						"action":             "experimental_scale_value",
						"experimental_scale": 1024 * 1024,
					})
				}
				for _, t := range metricDuplicateTypes {
					transformRules = append(transformRules, map[string]interface{}{
						"include":  old,
						"action":   "insert",
						"new_name": metricName(t, new),
						"operations": append([]map[string]interface{}{
							{
								"action":    "add_label",
								"new_label": constants.MetricType,
								"new_value": t,
							},
						}, operations...),
					})
				}
			}

			for oldName, newName := range renameMapForNeuronMonitor {
				var operations []map[string]interface{}
				if newName == constants.NeuronCoreUtilization {
					operations = append(operations, map[string]interface{}{
						"action":             "experimental_scale_value",
						"experimental_scale": 100,
					})
				}

				transformRules = append(transformRules, map[string]interface{}{
					"include":  oldName,
					"action":   "update",
					"new_name": newName,
					"operations": append([]map[string]interface{}{},
						operations...),
				})
			}
		}
	} else if t.name == common.PipelineNameJmx {
		transformRules = []map[string]interface{}{
			{
				"include": "tomcat.sessions",
				"action":  "update",
				"operations": []map[string]interface{}{
					{
						"action":           "aggregate_labels",
						"aggregation_type": "sum",
					},
					{
						"action": "delete_label_value",
						"label":  "context",
					},
				},
			},
			{
				"include": "tomcat.rejected_sessions",
				"action":  "update",
				"operations": []map[string]interface{}{
					{
						"action":           "aggregate_labels",
						"aggregation_type": "sum",
					},
					{
						"action": "delete_label_value",
						"label":  "context",
					},
				},
			},
		}
	}

	if len(transformRules) == 0 {
		return nil, fmt.Errorf("no transform rules for %s", t.name)
	}

	c := confmap.NewFromStringMap(map[string]interface{}{
		"transforms": transformRules,
	})
	if err := c.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal into metricstransform config: %w", err)
	}

	return cfg, nil
}

func metricName(mType string, name string) string {
	prefix := ""
	nodePrefix := "node_"
	podPrefix := "pod_"
	containerPrefix := "container_"
	cluster := "cluster_"

	switch mType {
	case constants.TypeContainer, constants.TypeGpuContainer:
		prefix = containerPrefix
	case constants.TypePod, constants.TypeGpuPod:
		prefix = podPrefix
	case constants.TypeNode, constants.TypeGpuNode:
		prefix = nodePrefix
	case constants.TypeCluster, constants.TypeGpuCluster:
		prefix = cluster
	default:
	}
	return prefix + name
}
