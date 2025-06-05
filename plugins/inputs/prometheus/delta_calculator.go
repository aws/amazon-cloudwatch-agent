// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"github.com/prometheus/prometheus/model/value"
	"log"
	"math"
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

	// Add debug logging to trace the metric value
	log.Printf("D! Processing metric: name=%s, nameBeforeRelabel=%s, value=%v, tags=%v, type=%s",
		pm.metricName,
		pm.metricNameBeforeRelabel,
		pm.metricValue,
		pm.tags,
		pm.metricType)

	if !pm.isValueValid() {
		log.Printf("D! Invalid value details: IsStaleNaN=%v, IsNaN=%v, IsInf=%v for metric %s (before relabel: %s)",
			value.IsStaleNaN(pm.metricValue),
			math.IsNaN(pm.metricValue),
			math.IsInf(pm.metricValue, 0),
			pm.metricName,
			pm.metricNameBeforeRelabel)
		log.Printf("D! Additional context - job: %s, instance: %s, jobBeforeRelabel: %s, instanceBeforeRelabel: %s",
			pm.tags["job"],
			pm.tags["instance"],
			pm.jobBeforeRelabel,
			pm.instanceBeforeRelabel)
		dc.preDataPoints.Delete(metricKey)
		return nil
	}

	curVal := pm.metricValue
	curTimeInMS := pm.timeInMS

	// Always set the result to pm initially
	res = pm

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
	} else {
		// For first data point, use the current value
		pm.metricValue = curVal
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
