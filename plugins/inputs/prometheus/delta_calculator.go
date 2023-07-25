// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"log"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/internal/mapWithExpiry"
)

const (
	CleanUpTimeThreshold = 60 * 1000       // 1 minute
	CacheTTL             = 5 * time.Minute // 5 minutes
)

type dataPoint struct {
	value    float64
	timeInMS int64
}
type DeltaCalculator struct {
	preDataPoints       *mapWithExpiry.MapWithExpiry
	lastCleanUpTimeInMs int64
}

func (dc *DeltaCalculator) calculate(pm *PrometheusMetric) (res *PrometheusMetric) {
	metricKey := getUniqMetricKey(pm)

	if !pm.isValueValid() {
		log.Printf("D! DeltaCalculator.calculate: Drop metric with NaN or Inf value: %v", pm)
		//When the raws values are like this: 1, 2, 3, 4, NaN, NaN, NaN, ..., 100, 101, 102,
		//and the previous value is not reset, we will get a wrong delta value (at 100) as 100 - 4 = 96
		//To avoid this issue, we reset the previous value whenever an invalid value is encountered
		dc.preDataPoints.Delete(metricKey)
		return nil
	}

	curVal := pm.metricValue
	curTimeInMS := pm.timeInMS
	if v, ok := dc.preDataPoints.Get(metricKey); ok {
		preDataPoint := v.(dataPoint)
		if curTimeInMS > preDataPoint.timeInMS {
			if curVal >= preDataPoint.value {
				pm.metricValue = curVal - preDataPoint.value
			} else {
				// the counter has been reset, keep the current value as delta
				pm.metricValue = curVal
			}
		}
		res = pm
	}

	// Clean up the stale cache periodically
	if curTimeInMS-dc.lastCleanUpTimeInMs >= CleanUpTimeThreshold {
		dc.preDataPoints.CleanUp(time.Now())
		dc.lastCleanUpTimeInMs = curTimeInMS
	}

	dc.preDataPoints.Set(metricKey, dataPoint{value: curVal, timeInMS: curTimeInMS})

	return
}

func NewDeltaCalculator() *DeltaCalculator {
	return &DeltaCalculator{preDataPoints: mapWithExpiry.NewMapWithExpiry(CacheTTL), lastCleanUpTimeInMs: 0}
}
