// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package gpuattributes

import (
	"context"
	"encoding/json"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/gpuattributes/internal"
)

const (
	gpuMetricIdentifier      = "_gpu_"
	gpuContainerMetricPrefix = "container_"
	gpuPodMetricPrefix       = "pod_"
	gpuNodeMetricPrefix      = "node_"
)

// schemas at each resource level
// - Container Schema
//   - ClusterName
//   - ClusterName, Namespace, PodName, ContainerName
//   - ClusterName, Namespace, PodName, FullPodName, ContainerName
//   - ClusterName, Namespace, PodName, FullPodName, ContainerName, GpuDevice
//
// - Pod
//   - ClusterName
//   - ClusterName, Namespace
//   - ClusterName, Namespace, Service
//   - ClusterName, Namespace, PodName
//   - ClusterName, Namespace, PodName, FullPodName
//   - ClusterName, Namespace, PodName, FullPodName, GpuDevice
//
// - Node
//   - ClusterName
//   - ClusterName, InstanceIdKey, NodeName
//   - ClusterName, InstanceIdKey, NodeName, GpuDevice
var containerLabelFilter = map[string]map[string]interface{}{
	containerinsightscommon.ClusterNameKey:   nil,
	containerinsightscommon.InstanceIdKey:    nil,
	containerinsightscommon.GpuDeviceKey:     nil,
	containerinsightscommon.MetricType:       nil,
	containerinsightscommon.NodeNameKey:      nil,
	containerinsightscommon.K8sNamespace:     nil,
	containerinsightscommon.FullPodNameKey:   nil,
	containerinsightscommon.PodNameKey:       nil,
	containerinsightscommon.TypeService:      nil,
	containerinsightscommon.GpuUniqueId:      nil,
	containerinsightscommon.ContainerNamekey: nil,
	containerinsightscommon.InstanceTypeKey:  nil,
	containerinsightscommon.VersionKey:       nil,
	containerinsightscommon.SourcesKey:       nil,
	containerinsightscommon.Timestamp:        nil,
	containerinsightscommon.K8sKey: {
		containerinsightscommon.HostKey: nil,
		"labels":                        nil,
		"pod_id":                        nil,
		"pod_name":                      nil,
		"pod_owners":                    nil,
		"namespace":                     nil,
		"container_name":                nil,
		"containerd":                    nil,
	},
}
var podLabelFilter = map[string]map[string]interface{}{
	containerinsightscommon.ClusterNameKey:  nil,
	containerinsightscommon.InstanceIdKey:   nil,
	containerinsightscommon.GpuDeviceKey:    nil,
	containerinsightscommon.MetricType:      nil,
	containerinsightscommon.NodeNameKey:     nil,
	containerinsightscommon.K8sNamespace:    nil,
	containerinsightscommon.FullPodNameKey:  nil,
	containerinsightscommon.PodNameKey:      nil,
	containerinsightscommon.TypeService:     nil,
	containerinsightscommon.GpuUniqueId:     nil,
	containerinsightscommon.InstanceTypeKey: nil,
	containerinsightscommon.VersionKey:      nil,
	containerinsightscommon.SourcesKey:      nil,
	containerinsightscommon.Timestamp:       nil,
	containerinsightscommon.K8sKey: {
		containerinsightscommon.HostKey: nil,
		"labels":                        nil,
		"pod_id":                        nil,
		"pod_name":                      nil,
		"pod_owners":                    nil,
		"namespace":                     nil,
	},
}
var nodeLabelFilter = map[string]map[string]interface{}{
	containerinsightscommon.ClusterNameKey:  nil,
	containerinsightscommon.InstanceIdKey:   nil,
	containerinsightscommon.GpuDeviceKey:    nil,
	containerinsightscommon.MetricType:      nil,
	containerinsightscommon.NodeNameKey:     nil,
	containerinsightscommon.InstanceTypeKey: nil,
	containerinsightscommon.VersionKey:      nil,
	containerinsightscommon.SourcesKey:      nil,
	containerinsightscommon.Timestamp:       nil,
	containerinsightscommon.K8sKey: {
		containerinsightscommon.HostKey: nil,
	},
}

type gpuAttributesProcessor struct {
	*Config
	logger                          *zap.Logger
	awsNeuronMetricModifier         *internal.AwsNeuronMetricModifier
	awsNeuronMemoryMetricAggregator *internal.AwsNeuronMemoryMetricsAggregator
}

func newGpuAttributesProcessor(config *Config, logger *zap.Logger) *gpuAttributesProcessor {
	d := &gpuAttributesProcessor{
		Config:                          config,
		logger:                          logger,
		awsNeuronMetricModifier:         internal.NewMetricModifier(logger),
		awsNeuronMemoryMetricAggregator: internal.NewMemoryMemoryAggregator(),
	}
	return d
}

func (d *gpuAttributesProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		ilms := rs.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			metrics := ils.Metrics()

			d.filterGpuMetricsWithoutPodName(metrics, rs.Resource().Attributes())

			metricsLength := metrics.Len()
			for k := 0; k < metricsLength; k++ {
				m := metrics.At(k)
				d.processGPUMetricAttributes(m)
				d.awsNeuronMemoryMetricAggregator.AggregateMemoryMetric(m)
				// non neuron metric is returned as a singleton list
				d.awsNeuronMetricModifier.ModifyMetric(m, metrics)
			}
			if d.awsNeuronMemoryMetricAggregator.MemoryMetricsFound {
				aggregatedMemoryMetric := d.awsNeuronMemoryMetricAggregator.FlushAggregatedMemoryMetric()
				d.awsNeuronMetricModifier.ModifyMetric(aggregatedMemoryMetric, metrics)
			}
		}
	}
	return md, nil
}

func (d *gpuAttributesProcessor) processGPUMetricAttributes(m pmetric.Metric) {
	// only decorate GPU metrics
	if !strings.Contains(m.Name(), gpuMetricIdentifier) {
		return
	}

	labelFilter := map[string]map[string]interface{}{}
	if strings.HasPrefix(m.Name(), gpuContainerMetricPrefix) {
		labelFilter = containerLabelFilter
	} else if strings.HasPrefix(m.Name(), gpuPodMetricPrefix) {
		labelFilter = podLabelFilter
	} else if strings.HasPrefix(m.Name(), gpuNodeMetricPrefix) {
		labelFilter = nodeLabelFilter
	}

	var dps pmetric.NumberDataPointSlice
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps = m.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dps = m.Sum().DataPoints()
	default:
		d.logger.Debug("Ignore unknown metric type", zap.String(containerinsightscommon.MetricType, m.Type().String()))
	}

	for i := 0; i < dps.Len(); i++ {
		d.filterAttributes(dps.At(i).Attributes(), labelFilter)
	}
}

func (d *gpuAttributesProcessor) filterAttributes(attributes pcommon.Map, labels map[string]map[string]interface{}) {
	if len(labels) == 0 {
		return
	}
	// remove labels that are not in the keep list
	attributes.RemoveIf(func(k string, _ pcommon.Value) bool {
		if _, ok := labels[k]; ok {
			return false
		}
		return true
	})

	// if a label has child level filter list, that means the label is map type
	// only handles map type since there are currently only map and value types with GPU
	for lk, ls := range labels {
		if len(ls) == 0 {
			continue
		}
		if av, ok := attributes.Get(lk); ok {
			// decode json formatted string value into a map then encode again after filtering elements
			var blob map[string]json.RawMessage
			strVal := av.Str()
			err := json.Unmarshal([]byte(strVal), &blob)
			if err != nil {
				d.logger.Warn("gpuAttributesProcessor: failed to unmarshal label", zap.String("label", lk))
				continue
			}
			newBlob := make(map[string]json.RawMessage)
			for bkey, bval := range blob {
				if _, ok := ls[bkey]; ok {
					newBlob[bkey] = bval
				}
			}
			bytes, err := json.Marshal(newBlob)
			if err != nil {
				d.logger.Warn("gpuAttributesProcessor: failed to marshall label", zap.String("label", lk))
				continue
			}
			attributes.PutStr(lk, string(bytes))
		}
	}
}

// remove dcgm metrics that do not contain PodName attribute which means there is no workload associated to container/pod
func (d *gpuAttributesProcessor) filterGpuMetricsWithoutPodName(metrics pmetric.MetricSlice, resourceAttributes pcommon.Map) {
	metrics.RemoveIf(func(m pmetric.Metric) bool {
		isGpu := strings.Contains(m.Name(), gpuMetricIdentifier)
		isContainerOrPod := strings.HasPrefix(m.Name(), gpuContainerMetricPrefix) || strings.HasPrefix(m.Name(), gpuPodMetricPrefix)
		if !isGpu || !isContainerOrPod {
			return false
		}

		_, hasPodAtResource := resourceAttributes.Get(internal.PodName)
		var dps pmetric.NumberDataPointSlice
		switch m.Type() {
		case pmetric.MetricTypeGauge:
			dps = m.Gauge().DataPoints()
		case pmetric.MetricTypeSum:
			dps = m.Sum().DataPoints()
		default:
			d.logger.Debug("Ignore unknown metric type", zap.String(containerinsightscommon.MetricType, m.Type().String()))
		}

		dps.RemoveIf(func(dp pmetric.NumberDataPoint) bool {
			_, hasPodInfo := dp.Attributes().Get(internal.PodName)
			return !hasPodInfo && !hasPodAtResource
		})
		return dps.Len() == 0
	})
}
