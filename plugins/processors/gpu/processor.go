// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package gpu

import (
	"context"
	"encoding/json"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	gpuMetric                = "_gpu_"
	gpuContainerMetricPrefix = "container_"
	gpuPodMetricPrefix       = "pod_"
	gpuNodeMetricPrefix      = "node_"
)

var podContainerMetricLabels = map[string]map[string]interface{}{
	"ClusterName":  nil,
	"FullPodName":  nil,
	"PodName":      nil,
	"InstanceId":   nil,
	"InstanceType": nil,
	"NodeName":     nil,
	"Timestamp":    nil,
	"Type":         nil,
	"Version":      nil,
	"Namespace":    nil,
	"Sources":      nil,
	"UUID":         nil,
	"kubernetes":   nil,
}

var nodeMetricLabels = map[string]map[string]interface{}{
	"ClusterName":  nil,
	"InstanceId":   nil,
	"InstanceType": nil,
	"NodeName":     nil,
	"Timestamp":    nil,
	"Type":         nil,
	"Version":      nil,
	"kubernetes": {
		"host": nil,
	},
}

type gpuprocessor struct {
	*Config
	logger     *zap.Logger
	cancelFunc context.CancelFunc
	shutdownC  chan bool
	started    bool
}

func newGpuProcessor(config *Config, logger *zap.Logger) *gpuprocessor {
	_, cancel := context.WithCancel(context.Background())
	d := &gpuprocessor{
		Config:     config,
		logger:     logger,
		cancelFunc: cancel,
	}
	return d
}

func (d *gpuprocessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	if !d.started {
		return pmetric.NewMetrics(), nil
	}

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		ilms := rs.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			metrics := ils.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				m := metrics.At(k)
				d.processMetricAttributes(ctx, m)
			}
		}
	}
	return md, nil
}

func (d *gpuprocessor) processMetricAttributes(_ context.Context, m pmetric.Metric) {
	// only decorate GPU metrics
	// another option is to separate GPU of its own pipeline to minimize extra processing of metrics
	if !strings.Contains(m.Name(), gpuMetric) {
		return
	}

	var labels map[string]map[string]interface{}
	if strings.HasPrefix(m.Name(), gpuNodeMetricPrefix) {
		labels = nodeMetricLabels
	} else if strings.HasPrefix(m.Name(), gpuContainerMetricPrefix) {
		labels = podContainerMetricLabels
		labels["kubernetes"] = map[string]interface{}{
			"container_name": nil,
			"containerd":     nil,
			"host":           nil,
			"labels":         nil,
			"pod_id":         nil,
			"pod_name":       nil,
			"pod_owners":     nil,
			"namespace":      nil,
		}
	} else if strings.HasPrefix(m.Name(), gpuPodMetricPrefix) {
		labels = podContainerMetricLabels
		labels["kubernetes"] = map[string]interface{}{
			"host":       nil,
			"labels":     nil,
			"pod_id":     nil,
			"pod_name":   nil,
			"pod_owners": nil,
			"namespace":  nil,
		}
	}

	var dps pmetric.NumberDataPointSlice
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps = m.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dps = m.Sum().DataPoints()
	default:
		d.logger.Debug("Ignore unknown metric type", zap.String("type", m.Type().String()))
	}

	for i := 0; i < dps.Len(); i++ {
		d.filterAttributes(dps.At(i).Attributes(), labels)
	}
}

func (d *gpuprocessor) filterAttributes(attributes pcommon.Map, labels map[string]map[string]interface{}) {
	if len(labels) < 1 {
		return
	}
	// remove labels that are no in the keep list
	attributes.RemoveIf(func(k string, _ pcommon.Value) bool {
		if _, ok := labels[k]; !ok {
			return true
		}
		return false
	})

	// if a label has child level filter list, that means the label is map type
	// only handles map type since there are currently only map and value types with GPU
	for lk, ls := range labels {
		if len(ls) < 1 {
			continue
		}
		if av, ok := attributes.Get(lk); ok {
			// decode json formatted string value into a map then encode again after filtering elements
			var blob map[string]json.RawMessage
			strVal := av.Str()
			err := json.Unmarshal([]byte(strVal), &blob)
			if err != nil {
				d.logger.Warn("gpuprocessor: failed to unmarshal label", zap.String("label", lk))
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
				d.logger.Warn("gpuprocessor: failed to marshall label", zap.String("label", lk))
				continue
			}
			attributes.PutStr(lk, string(bytes))
		}
	}
}

func (d *gpuprocessor) Shutdown(context.Context) error {
	close(d.shutdownC)
	d.cancelFunc()
	return nil
}

func (d *gpuprocessor) Start(ctx context.Context, _ component.Host) error {
	d.shutdownC = make(chan bool)
	d.started = true
	return nil
}
