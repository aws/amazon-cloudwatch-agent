// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package distribution

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

type HistogramInput struct {
	Count      uint64
	Sum        float64
	Min        *float64
	Max        *float64
	Boundaries []float64
	Counts     []uint64
	Attributes map[string]string
}

type ExpectedMetrics struct {
	Count       uint64
	Sum         float64
	Average     float64
	MedianRange struct {
		Low  float64
		High float64
	}
}

type HistogramTestCase struct {
	Name     string
	Input    HistogramInput
	Expected ExpectedMetrics
}

func getTestCases() []HistogramTestCase {

	// Create large bucket arrays with 11 items per bucket
	boundaries125 := make([]float64, 125)
	counts125 := make([]uint64, 126)
	for i := 0; i < 125; i++ {
		boundaries125[i] = float64(i+1) * 10
		counts125[i] = 11
	}
	counts125[125] = 11 // 1,386 total items

	boundaries175 := make([]float64, 175)
	counts175 := make([]uint64, 176)
	for i := 0; i < 175; i++ {
		boundaries175[i] = float64(i+1) * 10
		counts175[i] = 11
	}
	counts175[175] = 11 // 1,936 total items

	boundaries225 := make([]float64, 225)
	counts225 := make([]uint64, 226)
	for i := 0; i < 225; i++ {
		boundaries225[i] = float64(i+1) * 10
		counts225[i] = 11
	}
	counts225[225] = 11 // 2,486 total items

	boundaries325 := make([]float64, 325)
	counts325 := make([]uint64, 326)
	for i := 0; i < 325; i++ {
		boundaries325[i] = float64(i+1) * 10
		counts325[i] = 11
	}
	counts325[325] = 11 // 3,586 total items

	return []HistogramTestCase{
		{
			Name: "Basic Histogram",
			Input: HistogramInput{
				Count:      101,
				Sum:        6000,
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 75, 100, 150},
				Counts:     []uint64{21, 31, 25, 15, 7, 2},
				Attributes: map[string]string{"service.name": "payment-service"},
			},
			Expected: ExpectedMetrics{
				Count:   101,
				Sum:     6000,
				Average: 59.41,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  25.0,
					High: 50.0,
				},
			},
		},
		{
			Name: "No Buckets",
			Input: HistogramInput{
				Count:      51,
				Sum:        1000,
				Min:        ptr(5.0),
				Max:        ptr(75.0),
				Boundaries: []float64{},
				Counts:     []uint64{},
				Attributes: map[string]string{"service.name": "auth-service"},
			},
			Expected: ExpectedMetrics{
				Count:   51,
				Sum:     1000,
				Average: 19.61,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  5.0,
					High: 75.0,
				},
			},
		},
		{
			Name: "Single Bucket",
			Input: HistogramInput{
				Count:      31,
				Sum:        150,
				Min:        ptr(1.0),
				Max:        ptr(10.0),
				Boundaries: []float64{5},
				Counts:     []uint64{21, 10},
				Attributes: map[string]string{"service.name": "database"},
			},
			Expected: ExpectedMetrics{
				Count:   31,
				Sum:     150,
				Average: 4.84,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  1.0,
					High: 5.0,
				},
			},
		},
		{
			Name: "Zero Counts and Sparse Data",
			Input: HistogramInput{
				Count:      101,
				Sum:        25000,
				Min:        ptr(0.0),
				Max:        ptr(1500.0),
				Boundaries: []float64{10, 50, 100, 500, 1000},
				Counts:     []uint64{51, 0, 0, 39, 0, 11},
				Attributes: map[string]string{"service.name": "cache-service"},
			},
			Expected: ExpectedMetrics{
				Count:   101,
				Sum:     25000,
				Average: 247.52,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  0.0,
					High: 10.0,
				},
			},
		},
		{
			Name: "Large Numbers",
			Input: HistogramInput{
				Count:      1001,
				Sum:        100000000000,
				Min:        ptr(1000000.0),
				Max:        ptr(1000000000.0),
				Boundaries: []float64{1000000, 10000000, 50000000, 100000000, 500000000},
				Counts:     []uint64{201, 301, 249, 150, 50, 50},
				Attributes: map[string]string{"service.name": "batch-processor"},
			},
			Expected: ExpectedMetrics{
				Count:   1001,
				Sum:     100000000000,
				Average: 99900099.90,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  1000000.0,
					High: 10000000.0,
				},
			},
		},
		{
			Name: "Many Buckets",
			Input: HistogramInput{
				Count:      1124,
				Sum:        350000,
				Min:        ptr(1.0),
				Max:        ptr(1100.0),
				Boundaries: []float64{1, 5, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 200, 300, 400, 500, 600, 700, 800, 900, 1000},
				Counts:     []uint64{51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 53},
				Attributes: map[string]string{"service.name": "detailed-metrics"},
			},
			Expected: ExpectedMetrics{
				Count:   1111,
				Sum:     350000,
				Average: 315.03,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  90.0,
					High: 100.0,
				},
			},
		},
		{
			Name: "Very Small Numbers",
			Input: HistogramInput{
				Count:      101,
				Sum:        0.00015,
				Min:        ptr(0.0000001),
				Max:        ptr(0.000006),
				Boundaries: []float64{0.0000001, 0.000001, 0.000002, 0.000003, 0.000004, 0.000005},
				Counts:     []uint64{11, 21, 29, 20, 15, 4, 1},
				Attributes: map[string]string{"service.name": "micro-timing"},
			},
			Expected: ExpectedMetrics{
				Count:   101,
				Sum:     0.00015,
				Average: 0.00000149,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  0.000001,
					High: 0.000002,
				},
			},
		},
		{
			Name: "Negative Values",
			Input: HistogramInput{
				Count:      101,
				Sum:        -3000,
				Min:        ptr(-100.0),
				Max:        ptr(60.0),
				Boundaries: []float64{0, 10, 20, 30, 40, 50},
				Counts:     []uint64{61, 10, 10, 10, 5, 4, 1},
				Attributes: map[string]string{"service.name": "temperature-service"},
			},
			Expected: ExpectedMetrics{
				Count:   101,
				Sum:     -3000,
				Average: -29.70,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  -100.0,
					High: 0.0,
				},
			},
		},
		{
			Name: "No Min or Max",
			Input: HistogramInput{
				Count:      75,
				Sum:        3500,
				Min:        nil,
				Max:        nil,
				Boundaries: []float64{10, 50, 100, 200},
				Counts:     []uint64{15, 21, 24, 10, 5},
				Attributes: map[string]string{"service.name": "web-service"},
			},
			Expected: ExpectedMetrics{
				Count:   75,
				Sum:     3500,
				Average: 46.67,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  50.0,
					High: 100.0,
				},
			},
		},
		{
			Name: "Only Max Defined",
			Input: HistogramInput{
				Count:      101,
				Sum:        17500,
				Min:        nil,
				Max:        ptr(750.0),
				Boundaries: []float64{100, 200, 300, 400, 500},
				Counts:     []uint64{21, 31, 24, 15, 5, 5},
				Attributes: map[string]string{"service.name": "api-gateway"},
			},
			Expected: ExpectedMetrics{
				Count:   101,
				Sum:     17500,
				Average: 173.27,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  100.0,
					High: 200.0,
				},
			},
		},
		{
			Name: "Only Min Defined",
			Input: HistogramInput{
				Count:      51,
				Sum:        4000,
				Min:        ptr(25.0),
				Max:        nil,
				Boundaries: []float64{50, 100, 150},
				Counts:     []uint64{11, 21, 14, 5},
				Attributes: map[string]string{"service.name": "queue-service"},
			},
			Expected: ExpectedMetrics{
				Count:   51,
				Sum:     4000,
				Average: 78.43,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  50.0,
					High: 100.0,
				},
			},
		},
		{
			Name: "No Min/Max with Single Value",
			Input: HistogramInput{
				Count:      1,
				Sum:        100,
				Min:        nil,
				Max:        nil,
				Boundaries: []float64{50, 150},
				Counts:     []uint64{0, 1, 0},
				Attributes: map[string]string{"service.name": "singleton-service"},
			},
			Expected: ExpectedMetrics{
				Count:   1,
				Sum:     100,
				Average: 100.0,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  50.0,
					High: 150.0,
				},
			},
		},
		// >100 buckets will be used for testing request splitting in PMD path
		{
			Name: "126 Buckets",
			Input: HistogramInput{
				Count:      1386, // 126 buckets * 11 items each
				Sum:        870555,
				Min:        ptr(5.0),
				Max:        ptr(1300.0),
				Boundaries: boundaries125,
				Counts:     counts125,
				Attributes: map[string]string{"service.name": "many-buckets-125"},
			},
			Expected: ExpectedMetrics{
				Count:   1386,
				Sum:     870555,
				Average: 573.14,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  630.0,
					High: 640.0,
				},
			},
		},
		// >150 buckets will be used for testing request splitting in EMF path
		{
			Name: "176 Buckets",
			Input: HistogramInput{
				Count:      1936, // 176 buckets * 11 items each
				Sum:        1697000,
				Min:        ptr(5.0),
				Max:        ptr(1800.0),
				Boundaries: boundaries175,
				Counts:     counts175,
				Attributes: map[string]string{"service.name": "many-buckets-175"},
			},
			Expected: ExpectedMetrics{
				Count:   1936,
				Sum:     1557000,
				Average: 804.23,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  880.0,
					High: 890.0,
				},
			},
		},
		// PMD should split into 3 requests
		// EMF should split into 2 requests
		{
			Name: "225 Buckets",
			Input: HistogramInput{
				Count:      2486, // 226 buckets * 11 items each
				Sum:        2803750,
				Min:        ptr(5.0),
				Max:        ptr(2300.0),
				Boundaries: boundaries225,
				Counts:     counts225,
				Attributes: map[string]string{"service.name": "many-buckets-225"},
			},
			Expected: ExpectedMetrics{
				Count:   2486,
				Sum:     2803750,
				Average: 1027.25,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  1130.0,
					High: 1140.0,
				},
			},
		},
		// PMD should split into 4 requests
		// EMF should split into 3 requests
		{
			Name: "325 Buckets",
			Input: HistogramInput{
				Count:      3586, // 326 buckets * 11 items each
				Sum:        5830500,
				Min:        ptr(5.0),
				Max:        ptr(3300.0),
				Boundaries: boundaries325,
				Counts:     counts325,
				Attributes: map[string]string{"service.name": "many-buckets-325"},
			},
			Expected: ExpectedMetrics{
				Count:   3586,
				Sum:     5830500,
				Average: 1486.47,
				MedianRange: struct {
					Low  float64
					High float64
				}{
					Low:  1630.0,
					High: 1640.0,
				},
			},
		},
	}
}
func getInvalidTestCases() []HistogramTestCase {
	return []HistogramTestCase{
		{
			Name: "Boundaries Not Ascending",
			Input: HistogramInput{
				Count:      100,
				Sum:        5000,
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 40, 100, 150}, // 40 < 50
				Counts:     []uint64{20, 30, 25, 15, 8, 2},
				Attributes: map[string]string{"service.name": "invalid-boundaries"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Counts Length Mismatch",
			Input: HistogramInput{
				Count:      100,
				Sum:        5000,
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 75, 100},
				Counts:     []uint64{20, 30, 25, 15, 8, 2}, // Should be 5 counts for 4 boundaries
				Attributes: map[string]string{"service.name": "wrong-counts"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Total Count Mismatch",
			Input: HistogramInput{
				Count:      90, // Doesn't match sum of counts (100)
				Sum:        5000,
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 8, 2},
				Attributes: map[string]string{"service.name": "count-mismatch"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Min Greater Than First Boundary",
			Input: HistogramInput{
				Count:      100,
				Sum:        5000,
				Min:        ptr(30.0), // Greater than first boundary (25)
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 8, 2}, // Has counts in first bucket
				Attributes: map[string]string{"service.name": "invalid-min"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Max Less Than Last Boundary",
			Input: HistogramInput{
				Count:      100,
				Sum:        5000,
				Min:        ptr(10.0),
				Max:        ptr(140.0), // Less than last boundary (150)
				Boundaries: []float64{25, 50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 8, 2}, // Has counts in overflow bucket
				Attributes: map[string]string{"service.name": "invalid-max"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Sum Too Small",
			Input: HistogramInput{
				Count:      100,
				Sum:        100, // Too small given the boundaries and counts
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 8, 2},
				Attributes: map[string]string{"service.name": "small-sum"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Sum Too Large",
			Input: HistogramInput{
				Count:      100,
				Sum:        1000000, // Too large given the boundaries and counts
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 8, 2},
				Attributes: map[string]string{"service.name": "large-sum"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Only Counts No Boundaries",
			Input: HistogramInput{
				Count:      100,
				Sum:        5000,
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{},
				Counts:     []uint64{100}, // Can't have counts without boundaries
				Attributes: map[string]string{"service.name": "counts-no-boundaries"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Min in Second Bucket But Sum Too Low",
			Input: HistogramInput{
				Count:      100,
				Sum:        2000,      // This sum is too low given min is in second bucket
				Min:        ptr(60.0), // Min falls in second bucket (50,75]
				Max:        ptr(200.0),
				Boundaries: []float64{50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 10}, // 30 values must be at least 60 each in second bucket
				Attributes: map[string]string{"service.name": "invalid-min-bucket"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Max in Second-to-Last Bucket But Sum Too High",
			Input: HistogramInput{
				Count:      100,
				Sum:        10000, // This sum is too high given max is in second-to-last bucket
				Min:        ptr(10.0),
				Max:        ptr(90.0), // Max falls in second-to-last bucket (75,100]
				Boundaries: []float64{50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 10}, // No value can exceed 90
				Attributes: map[string]string{"service.name": "invalid-max-bucket"},
			},
			Expected: ExpectedMetrics{},
		},
	}
}

func ptr(f float64) *float64 {
	return &f
}

func checkFeasibility(hi HistogramInput) (bool, string) {
	// Special case: empty histogram is valid
	if len(hi.Boundaries) == 0 && len(hi.Counts) == 0 {
		return true, ""
	}

	// Either both boundaries and counts must be present, or both must be empty
	if (len(hi.Boundaries) == 0) != (len(hi.Counts) == 0) {
		return false, fmt.Sprintf("boundaries and counts must either both be present or both be empty. boundaries: %d, counts: %d",
			len(hi.Boundaries), len(hi.Counts))
	}

	// Rest of checks only apply if we have boundaries/counts
	if len(hi.Boundaries) > 0 || len(hi.Counts) > 0 {
		// Check boundaries are in ascending order
		for i := 1; i < len(hi.Boundaries); i++ {
			if hi.Boundaries[i] <= hi.Boundaries[i-1] {
				return false, fmt.Sprintf("boundaries not in ascending order: %v <= %v",
					hi.Boundaries[i], hi.Boundaries[i-1])
			}
		}

		// Check counts array length
		if len(hi.Counts) != len(hi.Boundaries)+1 {
			return false, fmt.Sprintf("counts length (%d) should be boundaries length (%d) + 1",
				len(hi.Counts), len(hi.Boundaries))
		}

		// Verify total count matches sum of bucket counts
		var totalCount uint64
		for _, count := range hi.Counts {
			totalCount += count
		}
		if totalCount != hi.Count {
			return false, fmt.Sprintf("sum of counts (%d) doesn't match total count (%d)",
				totalCount, hi.Count)
		}

		// Check min/max feasibility if defined
		if hi.Min != nil {
			// If there are boundaries, first bucket must have counts > 0 only if min <= first boundary
			if len(hi.Boundaries) > 0 && hi.Counts[0] > 0 && *hi.Min > hi.Boundaries[0] {
				return false, fmt.Sprintf("min (%v) > first boundary (%v) but first bucket has counts",
					*hi.Min, hi.Boundaries[0])
			}
		}

		if hi.Max != nil {
			// If there are boundaries, last bucket must have counts > 0 only if max > last boundary
			if len(hi.Boundaries) > 0 && hi.Counts[len(hi.Counts)-1] > 0 &&
				*hi.Max <= hi.Boundaries[len(hi.Boundaries)-1] {
				return false, fmt.Sprintf("max (%v) <= last boundary (%v) but overflow bucket has counts",
					*hi.Max, hi.Boundaries[len(hi.Boundaries)-1])
			}
		}

		// Check sum feasibility
		if len(hi.Boundaries) > 0 {
			// Calculate minimum possible sum
			minSum := float64(0)
			if hi.Min != nil {
				// Find which bucket the minimum value belongs to
				minBucket := 0
				for i, bound := range hi.Boundaries {
					if *hi.Min > bound {
						minBucket = i + 1
					}
				}
				// Apply min value only from its containing bucket
				for i := minBucket; i < len(hi.Counts); i++ {
					if i == minBucket {
						minSum += float64(hi.Counts[i]) * *hi.Min
					} else {
						minSum += float64(hi.Counts[i]) * hi.Boundaries[i-1]
					}
				}
			} else {
				// Without min, use lower bounds
				for i := 1; i < len(hi.Counts); i++ {
					minSum += float64(hi.Counts[i]) * hi.Boundaries[i-1]
				}
			}

			// Calculate maximum possible sum
			maxSum := float64(0)
			if hi.Max != nil {
				// Find which bucket the maximum value belongs to
				maxBucket := len(hi.Boundaries) // Default to overflow bucket
				for i, bound := range hi.Boundaries {
					if *hi.Max <= bound {
						maxBucket = i
						break
					}
				}
				// Apply max value only up to its containing bucket
				for i := 0; i < len(hi.Counts); i++ {
					if i > maxBucket {
						maxSum += float64(hi.Counts[i]) * *hi.Max
					} else if i == len(hi.Boundaries) {
						maxSum += float64(hi.Counts[i]) * *hi.Max
					} else {
						maxSum += float64(hi.Counts[i]) * hi.Boundaries[i]
					}
				}
			} else {
				// If no max defined, we can't verify upper bound
				maxSum = math.Inf(1)
			}

			if hi.Sum < minSum {
				return false, fmt.Sprintf("sum (%v) is less than minimum possible sum (%v)",
					hi.Sum, minSum)
			}
			if maxSum != math.Inf(1) && hi.Sum > maxSum {
				return false, fmt.Sprintf("sum (%v) is greater than maximum possible sum (%v)",
					hi.Sum, maxSum)
			}
		}
	}

	return true, ""
}

func calculateMedianRange(hi HistogramInput) (float64, float64) {
	if len(hi.Boundaries) == 0 {
		// No buckets - use min/max if available
		if hi.Min != nil && hi.Max != nil {
			return *hi.Min, *hi.Max
		}
		return 0, 0
	}

	medianPosition := hi.Count / 2
	var cumulativeCount uint64

	// Find which bucket contains the median
	for i, count := range hi.Counts {
		cumulativeCount += count
		if cumulativeCount > medianPosition {
			// Found the bucket containing the median
			if i == 0 {
				// First bucket: (-inf, bounds[0]]
				if hi.Min != nil {
					return *hi.Min, hi.Boundaries[0]
				}
				return math.Inf(-1), hi.Boundaries[0]
			} else if i == len(hi.Boundaries) {
				// Last bucket: (bounds[last], +inf)
				if hi.Max != nil {
					return hi.Boundaries[i-1], *hi.Max
				}
				return hi.Boundaries[i-1], math.Inf(1)
			} else {
				// Middle bucket: (bounds[i-1], bounds[i]]
				return hi.Boundaries[i-1], hi.Boundaries[i]
			}
		}
	}
	return 0, 0 // Should never reach here for valid histograms
}

func TestHistogramFeasibility(t *testing.T) {
	testCases := getTestCases()
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			feasible, reason := checkFeasibility(tc.Input)
			assert.True(t, feasible, reason)

			// check that the test case median range is valid
			calculatedLow, calculatedHigh := calculateMedianRange(tc.Input)
			assert.Equal(t, calculatedLow, tc.Expected.MedianRange.Low, "calculated low does not match expected low. check test definition")
			assert.Equal(t, calculatedHigh, tc.Expected.MedianRange.High, "calculated high does not match expected high. check test definition")
		})
	}
}

func TestInvalidHistogramFeasibility(t *testing.T) {
	invalidTestCases := getInvalidTestCases()

	for _, tc := range invalidTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			feasible, reason := checkFeasibility(tc.Input)
			assert.False(t, feasible, reason)
		})
	}
}
