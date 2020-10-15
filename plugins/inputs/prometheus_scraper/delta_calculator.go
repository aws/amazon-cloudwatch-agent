// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus_scraper

import (
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

	if pm.isValueStale() {
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
