// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cardinalitycontrol

import (
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

var metricNames = []string{"latency", "error", "fault"}

func TestUpdateFrequency(t *testing.T) {
	cms := NewCountMinSketch(3, 10)
	for i := 0; i < 10; i++ {
		md := MetricData{
			hashKey:   "xxx",
			name:      "latency",
			service:   "app1",
			frequency: 1,
		}
		cms.Insert(md)
		val := cms.Get(md)
		assert.Equal(t, 1+i, val)
	}
}

var testCases = []int{50, 100, 200, 500, 1000, 2000}

func TestWriteMultipleEntries(t *testing.T) {
	cms := NewCountMinSketch(3, 5000)

	maxCollisionRate := 0
	for _, dataCount := range testCases {
		metricDataArray := make([]*MetricData, dataCount)
		for i := 0; i < dataCount; i++ {
			labels := map[string]string{
				"operation": "/api/customers/" + strconv.Itoa(rand.Int()),
			}
			for _, metricName := range metricNames {
				freq := rand.Intn(5000)
				md := MetricData{
					hashKey:   sortAndConcatLabels(labels),
					name:      metricName,
					service:   "app",
					frequency: freq,
				}
				cms.Insert(md)
				if metricDataArray[i] == nil {
					metricDataArray[i] = &md
				} else {
					metricDataArray[i].frequency = metricDataArray[i].frequency + freq
				}

			}
		}

		err := 0
		for _, data := range metricDataArray {
			val := cms.Get(data)
			if data.frequency != val {
				err += 1
			}
		}
		collisionRate := err * 100 / len(metricDataArray)
		if maxCollisionRate < collisionRate {
			maxCollisionRate = collisionRate
		}
		t.Logf("When the item count is %d with even distribution, the collision rate is %d.\n", dataCount, collisionRate)
	}

	// revisit the count min sketch setting if the assertion fails.
	assert.True(t, maxCollisionRate < 30)
}

func TestAdjustUnsupportedDepth(t *testing.T) {
	cms := NewCountMinSketch(5, 10)
	assert.Equal(t, 3, cms.depth)
	for i := 0; i < 2; i++ {
		cms.RegisterHashFunc(func(hashKey string) int64 {
			return int64(0)
		})
	}
	assert.Equal(t, 5, cms.depth)
	for i := 0; i < 2; i++ {
		cms.RegisterHashFunc(func(hashKey string) int64 {
			return int64(0)
		})
	}
	assert.Equal(t, 5, cms.depth)
}
