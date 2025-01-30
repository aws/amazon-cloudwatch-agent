// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrichandlers

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type aggregationType int

const (
	defaultAggregation aggregationType = iota
	lastValueAggregation
)

// AggregationMutator is used to convert predefined ObservableUpDownCounter metrics to use LastValue metrichandlers. This
// is necessary for cases where metrics are instrumented as cumulative, yet reported with snapshot values.
//
// For example, metrics like DotNetGCGen0HeapSize may report values such as 1000, 2000, 1000, with cumulative temporality
// When exporters, such as the EMF exporter, detect these as cumulative, they convert the values to deltas,
// resulting in outputs like -, 1000, -1000, which misrepresent the data.
//
// Normally, this issue could be resolved by configuring a view with LastValue metrichandlers within the SDK.
// However, since the view feature is not fully supported in .NET, this workaround implements the required
// conversion to LastValue metrichandlers to ensure accurate metric reporting.
// See https://github.com/open-telemetry/opentelemetry-dotnet/issues/2618.
type AggregationMutator struct {
	includes map[string]aggregationType
}

func NewAggregationMutator() AggregationMutator {
	return newAggregationMutatorWithConfig(map[string]aggregationType{
		"DotNetGCGen0HeapSize":    lastValueAggregation,
		"DotNetGCGen1HeapSize":    lastValueAggregation,
		"DotNetGCGen2HeapSize":    lastValueAggregation,
		"DotNetGCLOHHeapSize":     lastValueAggregation,
		"DotNetGCPOHHeapSize":     lastValueAggregation,
		"DotNetThreadCount":       lastValueAggregation,
		"DotNetThreadQueueLength": lastValueAggregation,
	})
}

func newAggregationMutatorWithConfig(includes map[string]aggregationType) AggregationMutator {
	return AggregationMutator{
		includes,
	}
}

func (t *AggregationMutator) ProcessMetrics(_ context.Context, m pmetric.Metric, _ pcommon.Map) {
	aggType, exists := t.includes[m.Name()]
	if !exists || aggType == defaultAggregation {
		return
	}
	switch m.Type() {
	case pmetric.MetricTypeSum:
		switch aggType {
		case lastValueAggregation:
			m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		default:
		}
	default:
	}
}
