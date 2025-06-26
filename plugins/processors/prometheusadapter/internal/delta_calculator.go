// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"log"
	"math"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/mapWithExpiry"
	"github.com/prometheus/prometheus/model/value"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	CleanUpTimeThreshold = 60 * 1000       // 1 minute
	CacheTTL             = 5 * time.Minute // 5 minutes
)

type dataPoint struct {
	value     float64
	timestamp pcommon.Timestamp
}
type DeltaCalculator struct {
	preDataPoints   *mapWithExpiry.MapWithExpiry
	lastCleanUpTime pcommon.Timestamp
}

func (dc *DeltaCalculator) Calculate(m pmetric.Metric) {

	switch m.Type() {
	case pmetric.MetricTypeSum:
		// only calculate delta if it's a cumulative sum metric
		if m.Sum().AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
			return
		}

		dps := m.Sum().DataPoints()
		dps.RemoveIf(func(dp pmetric.NumberDataPoint) bool {
			identity := MetricIdentity{
				name: m.Name(),
				tags: dp.Attributes().AsRaw(),
			}
			metricKey := identity.getKey()

			var curVal float64
			switch dp.ValueType() {
			case pmetric.NumberDataPointValueTypeInt:
				curVal = float64(dp.IntValue())
			case pmetric.NumberDataPointValueTypeDouble:
				curVal = dp.DoubleValue()
			case pmetric.NumberDataPointValueTypeEmpty:
				fallthrough
			default:
				// cannot handle the type so drop the data point
				log.Printf("D! DeltaCalculator.calculate: Drop metric with value type: %v", dp.ValueType())
				return true
			}

			if !isValueValid(dp.DoubleValue()) {
				log.Printf("D! DeltaCalculator.calculate: Drop metric with NaN or Inf value: %v", curVal)
				//When the raws values are like this: 1, 2, 3, 4, NaN, NaN, NaN, ..., 100, 101, 102,
				//and the previous value is not reset, we will get a wrong delta value (at 100) as 100 - 4 = 96
				//To avoid this issue, we reset the previous value whenever an invalid value is encountered
				dc.preDataPoints.Delete(metricKey)
				return true
			}

			curTime := dp.Timestamp()

			dropValue := false
			if newVal, ok := dc.calculateDatapoint(metricKey, curVal, curTime); ok {
				switch dp.ValueType() {
				case pmetric.NumberDataPointValueTypeInt:
					dp.SetIntValue(int64(newVal))
				case pmetric.NumberDataPointValueTypeDouble:
					dp.SetDoubleValue(newVal)
				case pmetric.NumberDataPointValueTypeEmpty:
					fallthrough
				default:
					log.Printf("D! DeltaCalculator.calculate: Drop metric with value type: %v", dp.ValueType())
					return true
				}
			} else {
				// Drop the initial value for delta calculations
				dropValue = true
			}

			// Clean up the stale cache periodically
			if curTime-dc.lastCleanUpTime >= CleanUpTimeThreshold {
				dc.preDataPoints.CleanUp(time.Now())
				dc.lastCleanUpTime = curTime
			}

			dc.preDataPoints.Set(metricKey, dataPoint{value: curVal, timestamp: curTime})

			return dropValue
		})

	case pmetric.MetricTypeSummary:
		dps := m.Summary().DataPoints()
		dps.RemoveIf(func(dp pmetric.SummaryDataPoint) bool {
			sumIdentity := MetricIdentity{
				name: m.Name() + "_sum",
				tags: dp.Attributes().AsRaw(),
			}
			sumKey := sumIdentity.getKey()

			curSum := dp.Sum()
			if !isValueValid(curSum) {
				log.Printf("D! DeltaCalculator.calculate: Drop metric with NaN or Inf value: %v", curSum)
				//When the raws values are like this: 1, 2, 3, 4, NaN, NaN, NaN, ..., 100, 101, 102,
				//and the previous value is not reset, we will get a wrong delta value (at 100) as 100 - 4 = 96
				//To avoid this issue, we reset the previous value whenever an invalid value is encountered
				dc.preDataPoints.Delete(sumKey)
				return true
			}

			curTime := dp.Timestamp()
			curCount := float64(dp.Count())
			countIdentity := MetricIdentity{
				name: m.Name() + "_count",
				tags: dp.Attributes().AsRaw(),
			}
			countKey := countIdentity.getKey()

			dropValue := false
			if newVal, ok := dc.calculateDatapoint(sumKey, curSum, curTime); ok {
				dp.SetSum(newVal)
			} else {
				dropValue = true
			}
			if newVal, ok := dc.calculateDatapoint(countKey, curCount, curTime); ok {
				dp.SetCount(uint64(newVal))
			} else {
				// we shouldn't ever see sum without count, but drop the entire summary if either are not present
				dropValue = true
			}

			// Clean up the stale cache periodically
			if curTime-dc.lastCleanUpTime >= CleanUpTimeThreshold {
				dc.preDataPoints.CleanUp(time.Now())
				dc.lastCleanUpTime = curTime
			}

			dc.preDataPoints.Set(countKey, dataPoint{value: curCount, timestamp: curTime})
			dc.preDataPoints.Set(sumKey, dataPoint{value: curSum, timestamp: curTime})

			return dropValue
		})

	case pmetric.MetricTypeEmpty, pmetric.MetricTypeGauge, pmetric.MetricTypeExponentialHistogram:
		fallthrough
	default:
		log.Printf("W! DeltaCalculator.Calculate: ignoring metric %s", m.Name())
		return
	}

}

func (dc *DeltaCalculator) calculateDatapoint(key string, value float64, curTime pcommon.Timestamp) (float64, bool) {
	if v, ok := dc.preDataPoints.Get(key); ok {
		preDataPoint := v.(dataPoint)
		newVal := value
		if curTime > preDataPoint.timestamp {
			if value >= preDataPoint.value {
				newVal = value - preDataPoint.value
			} else {
				// the counter has been reset, keep the current value as delta
			}
		}
		return newVal, true
	}
	return value, false
}

func NewDeltaCalculator() *DeltaCalculator {
	return &DeltaCalculator{preDataPoints: mapWithExpiry.NewMapWithExpiry(CacheTTL), lastCleanUpTime: 0}
}

func isValueValid(v float64) bool {
	//treat NaN and +/-Inf values as invalid as emf log doesn't support them
	return !value.IsStaleNaN(v) && !math.IsNaN(v) && !math.IsInf(v, 0)
}
