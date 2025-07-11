// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package kueueattributes

import (
	"context"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent/internal/constants"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	kueueMetricsIdentifier = "kueue"
)

var kueueLabelFilter = map[string]interface{}{
	constants.ClusterNameKey:          nil,
	constants.ClusterQueueNameKey:     nil,
	constants.ClusterQueueStatusKey:   nil,
	constants.ClusterQueueReasonKey:   nil,
	constants.ClusterQueueResourceKey: nil,
	constants.Flavor:                  nil,
	constants.NodeNameKey:             nil,
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
		rm := rms.At(i)
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			metrics := sms.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				m := metrics.At(k)
				d.processMetricAttributes(m)
			}
		}
		d.dropResourceMetricAttributes(rm)
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
		d.logger.Debug("Ignore unknown metric type", zap.String(constants.MetricType, m.Type().String()))
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

func (d *kueueAttributesProcessor) dropResourceMetricAttributes(resourceMetric pmetric.ResourceMetrics) {
	serviceNameKey := "service.name"
	attributes := resourceMetric.Resource().Attributes()
	serviceName, exists := attributes.Get(serviceNameKey)

	if exists && (serviceName.Str() == "containerInsightsKueueMetricsScraper") {
		resourceMetric.Resource().Attributes().Clear()
	}
}
