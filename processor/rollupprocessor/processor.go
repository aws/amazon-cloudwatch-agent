// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rollupprocessor

import (
	"context"
	"sort"

	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"golang.org/x/exp/maps"

	"github.com/aws/amazon-cloudwatch-agent/internal/metric"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

type rollupProcessor struct {
	attributeGroups [][]string
	dropOriginal    collections.Set[string]
	cache           rollupCache
}

func newProcessor(cfg *Config) *rollupProcessor {
	cacheSize := cfg.CacheSize
	// use no-op cache if no attribute groups
	if len(cfg.AttributeGroups) == 0 {
		cacheSize = 0
	}
	return &rollupProcessor{
		attributeGroups: uniqueGroups(cfg.AttributeGroups),
		dropOriginal:    collections.NewSet(cfg.DropOriginal...),
		cache:           buildRollupCache(cacheSize),
	}
}

func (p *rollupProcessor) start(context.Context, component.Host) error {
	go p.cache.Start()
	return nil
}

func (p *rollupProcessor) stop(context.Context) error {
	p.cache.Stop()
	return nil
}

func (p *rollupProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	if len(p.attributeGroups) > 0 || len(p.dropOriginal) > 0 {
		metric.RangeMetrics(md, p.processMetric)
	}
	return md, nil
}

func (p *rollupProcessor) processMetric(m pmetric.Metric) {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		newDataPoints := pmetric.NewNumberDataPointSlice()
		rollupDataPoints[pmetric.NumberDataPoint](
			p.cache,
			p.attributeGroups,
			p.dropOriginal,
			m.Name(),
			m.Gauge().DataPoints(),
			newDataPoints,
		)
		newDataPoints.CopyTo(m.Gauge().DataPoints())
	case pmetric.MetricTypeSum:
		newDataPoints := pmetric.NewNumberDataPointSlice()
		rollupDataPoints[pmetric.NumberDataPoint](
			p.cache,
			p.attributeGroups,
			p.dropOriginal,
			m.Name(),
			m.Sum().DataPoints(),
			newDataPoints,
		)
		newDataPoints.CopyTo(m.Sum().DataPoints())
	case pmetric.MetricTypeHistogram:
		newDataPoints := pmetric.NewHistogramDataPointSlice()
		rollupDataPoints[pmetric.HistogramDataPoint](
			p.cache,
			p.attributeGroups,
			p.dropOriginal,
			m.Name(),
			m.Histogram().DataPoints(),
			newDataPoints,
		)
		newDataPoints.CopyTo(m.Histogram().DataPoints())
	case pmetric.MetricTypeExponentialHistogram:
		newDataPoints := pmetric.NewExponentialHistogramDataPointSlice()
		rollupDataPoints[pmetric.ExponentialHistogramDataPoint](
			p.cache,
			p.attributeGroups,
			p.dropOriginal,
			m.Name(),
			m.ExponentialHistogram().DataPoints(),
			newDataPoints,
		)
		newDataPoints.CopyTo(m.ExponentialHistogram().DataPoints())
	case pmetric.MetricTypeSummary:
		newDataPoints := pmetric.NewSummaryDataPointSlice()
		rollupDataPoints[pmetric.SummaryDataPoint](
			p.cache,
			p.attributeGroups,
			p.dropOriginal,
			m.Name(),
			m.Summary().DataPoints(),
			newDataPoints,
		)
		newDataPoints.CopyTo(m.Summary().DataPoints())
	}
}

// rollupDataPoints makes copies of the original data points for each rollup
// attribute group. If the metric name is in the drop original set, the original
// data points are dropped.
func rollupDataPoints[T metric.DataPoint[T]](
	cache rollupCache,
	attributeGroups [][]string,
	dropOriginal collections.Set[string],
	metricName string,
	orig metric.DataPoints[T],
	dest metric.DataPoints[T],
) {
	metric.RangeDataPoints(orig, func(origDataPoint T) {
		if !dropOriginal.Contains(metricName) {
			origDataPoint.CopyTo(dest.AppendEmpty())
		}
		if len(attributeGroups) == 0 {
			return
		}
		key := cache.Key(origDataPoint.Attributes())
		item := cache.Get(key)
		var rollup []pcommon.Map
		if item == nil {
			rollup = buildRollup(attributeGroups, origDataPoint.Attributes())
			cache.Set(key, rollup, ttlcache.DefaultTTL)
		} else {
			rollup = item.Value()
		}
		for _, attrs := range rollup {
			destDataPoint := dest.AppendEmpty()
			origDataPoint.CopyTo(destDataPoint)
			attrs.CopyTo(destDataPoint.Attributes())
		}
	})
}

func buildRollup(attributeGroups [][]string, baseAttributes pcommon.Map) []pcommon.Map {
	var results []pcommon.Map
	for _, rollupGroup := range attributeGroups {
		// skip if target dimensions count is same or more than the original metric.
		// cannot have dimensions that do not exist in the original metric.
		if len(rollupGroup) >= baseAttributes.Len() {
			continue
		}
		attributes := pcommon.NewMap()
		attributes.EnsureCapacity(len(rollupGroup))
		for _, key := range rollupGroup {
			value, ok := baseAttributes.Get(key)
			if !ok {
				break
			}
			value.CopyTo(attributes.PutEmpty(key))
		}
		if attributes.Len() == len(rollupGroup) {
			results = append(results, attributes)
		}
	}
	return results
}

// uniqueGroups filters out duplicate attributes within the sets and filters
// duplicate sets.
func uniqueGroups(groups [][]string) [][]string {
	if len(groups) == 0 {
		return nil
	}
	var results [][]string
	var uniqueSets []collections.Set[string]
	for _, rollupGroup := range groups {
		rollupSet := collections.NewSet(rollupGroup...)
		isUnique := collections.Range(uniqueSets, func(u collections.Set[string]) bool {
			return !rollupSet.Equal(u)
		})
		if isUnique {
			keys := maps.Keys(rollupSet)
			sort.Strings(keys)
			results = append(results, keys)
			uniqueSets = append(uniqueSets, rollupSet)
		}
	}
	return results
}
