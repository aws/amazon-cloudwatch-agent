// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package kueueattributes

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
)

const (
	kueueMetricsIdentifier = "kueue"
)

var kueueLabelFilter = map[string]interface{}{
	containerinsightscommon.ClusterNameKey:          nil,
	containerinsightscommon.ClusterQueueNameKey:     nil,
	containerinsightscommon.ClusterQueueStatusKey:   nil,
	containerinsightscommon.ClusterQueueReasonKey:   nil,
	containerinsightscommon.ClusterQueueResourceKey: nil,
	containerinsightscommon.Flavor:                  nil,
}

type kueueAttributesProcessor struct {
	*Config
	logger      *zap.Logger
	labelFilter map[string]interface{}
}

func newKueueAttributesProcessor(config *Config, logger *zap.Logger) *kueueAttributesProcessor {
	d := &kueueAttributesProcessor{
		Config:      config,
		logger:      logger,
		labelFilter: kueueLabelFilter,
	}
	return d
}

func (d *kueueAttributesProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			metrics := sms.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				m := metrics.At(k)
				d.processMetricAttributes(m)
			}
		}
	}
	return md, nil
}

func (d *kueueAttributesProcessor) processMetricAttributes(m pmetric.Metric) {

	// only decorate kueue metrics
	if !strings.HasPrefix(m.Name(), kueueMetricsIdentifier) {
		return
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
		d.filterAttributes(dps.At(i).Attributes())
	}
}

func (d *kueueAttributesProcessor) filterAttributes(attributes pcommon.Map) {
	labels := d.labelFilter
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
}
