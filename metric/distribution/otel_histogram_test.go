// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package distribution

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

type HistogramTestCase struct {
	Name       string
	Count      uint64
	Sum        float64
	Min        *float64
	Max        *float64
	Boundaries []float64
	Counts     []uint64
	Attributes map[string]string
}

func getTestCases() []HistogramTestCase {
	return []HistogramTestCase{
		{
			Name:       "Basic Histogram",
			Count:      100,
			Sum:        6000,
			Min:        ptr(10.0),
			Max:        ptr(200.0),
			Boundaries: []float64{25, 50, 75, 100, 150},
			Counts:     []uint64{20, 30, 25, 15, 8, 2},
			Attributes: map[string]string{"service.name": "payment-service"},
		},
		{
			Name:       "No Buckets",
			Count:      50,
			Sum:        1000,
			Min:        ptr(5.0),
			Max:        ptr(75.0),
			Boundaries: []float64{},
			Counts:     []uint64{},
			Attributes: map[string]string{"service.name": "auth-service"},
		},
		{
			Name:       "Single Bucket",
			Count:      30,
			Sum:        150,
			Min:        ptr(1.0),
			Max:        ptr(10.0),
			Boundaries: []float64{5},
			Counts:     []uint64{20, 10},
			Attributes: map[string]string{"service.name": "database"},
		},
		{
			Name:       "Zero Counts and Sparse Data",
			Count:      100,
			Sum:        25000,
			Min:        ptr(0.0),
			Max:        ptr(1500.0),
			Boundaries: []float64{10, 50, 100, 500, 1000},
			Counts:     []uint64{50, 0, 0, 40, 0, 10},
			Attributes: map[string]string{"service.name": "cache-service"},
		},
		{
			Name:       "Large Numbers",
			Count:      1000,
			Sum:        100000000000,
			Min:        ptr(1000000.0),
			Max:        ptr(1000000000.0),
			Boundaries: []float64{1000000, 10000000, 50000000, 100000000, 500000000},
			Counts:     []uint64{200, 300, 250, 150, 50, 50},
			Attributes: map[string]string{"service.name": "batch-processor"},
		},
		{
			Name:       "Many Buckets",
			Count:      1100,
			Sum:        350000, // Reduced from 550000 to be within maximum possible sum
			Min:        ptr(1.0),
			Max:        ptr(1100.0),
			Boundaries: []float64{1, 5, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 200, 300, 400, 500, 600, 700, 800, 900, 1000},
			Counts:     []uint64{50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50},
			Attributes: map[string]string{"service.name": "detailed-metrics"},
		},
		{
			Name:       "Very Small Numbers",
			Count:      100,
			Sum:        0.00015, // Increased from 0.000015 to meet minimum possible sum
			Min:        ptr(0.0000001),
			Max:        ptr(0.000006),
			Boundaries: []float64{0.0000001, 0.000001, 0.000002, 0.000003, 0.000004, 0.000005},
			Counts:     []uint64{10, 20, 30, 20, 15, 4, 1},
			Attributes: map[string]string{"service.name": "micro-timing"},
		},
		{
			Name:       "Negative Values",
			Count:      100,
			Sum:        -3000,
			Min:        ptr(-100.0),
			Max:        ptr(60.0),
			Boundaries: []float64{0, 10, 20, 30, 40, 50},
			Counts:     []uint64{60, 10, 10, 10, 5, 4, 1},
			Attributes: map[string]string{"service.name": "temperature-service"},
		},
		{
			Name:       "No Min or Max",
			Count:      75,
			Sum:        3500,
			Min:        nil,
			Max:        nil,
			Boundaries: []float64{10, 50, 100, 200},
			Counts:     []uint64{15, 20, 25, 10, 5},
			Attributes: map[string]string{"service.name": "web-service"},
		},
		{
			Name:       "Only Max Defined",
			Count:      100,
			Sum:        17500,
			Min:        nil,
			Max:        ptr(750.0),
			Boundaries: []float64{100, 200, 300, 400, 500},
			Counts:     []uint64{20, 30, 25, 15, 5, 5},
			Attributes: map[string]string{"service.name": "api-gateway"},
		},
		{
			Name:       "Only Min Defined",
			Count:      50,
			Sum:        4000,
			Min:        ptr(25.0),
			Max:        nil,
			Boundaries: []float64{50, 100, 150},
			Counts:     []uint64{10, 20, 15, 5},
			Attributes: map[string]string{"service.name": "queue-service"},
		},
		{
			Name:       "No Min/Max with Single Value",
			Count:      1,
			Sum:        100,
			Min:        nil,
			Max:        nil,
			Boundaries: []float64{50, 150},
			Counts:     []uint64{0, 1, 0},
			Attributes: map[string]string{"service.name": "singleton-service"},
		},
	}
}

func getInvalidTestCases() []HistogramTestCase {
	return []HistogramTestCase{
		{
			Name:       "Boundaries Not Ascending",
			Count:      100,
			Sum:        5000,
			Min:        ptr(10.0),
			Max:        ptr(200.0),
			Boundaries: []float64{25, 50, 40, 100, 150}, // 40 < 50
			Counts:     []uint64{20, 30, 25, 15, 8, 2},
			Attributes: map[string]string{"service.name": "invalid-boundaries"},
		},
		{
			Name:       "Counts Length Mismatch",
			Count:      100,
			Sum:        5000,
			Min:        ptr(10.0),
			Max:        ptr(200.0),
			Boundaries: []float64{25, 50, 75, 100},
			Counts:     []uint64{20, 30, 25, 15, 8, 2}, // Should be 5 counts for 4 boundaries
			Attributes: map[string]string{"service.name": "wrong-counts"},
		},
		{
			Name:       "Total Count Mismatch",
			Count:      90, // Doesn't match sum of counts (100)
			Sum:        5000,
			Min:        ptr(10.0),
			Max:        ptr(200.0),
			Boundaries: []float64{25, 50, 75, 100, 150},
			Counts:     []uint64{20, 30, 25, 15, 8, 2},
			Attributes: map[string]string{"service.name": "count-mismatch"},
		},
		{
			Name:       "Min Greater Than First Boundary",
			Count:      100,
			Sum:        5000,
			Min:        ptr(30.0), // Greater than first boundary (25)
			Max:        ptr(200.0),
			Boundaries: []float64{25, 50, 75, 100, 150},
			Counts:     []uint64{20, 30, 25, 15, 8, 2}, // Has counts in first bucket
			Attributes: map[string]string{"service.name": "invalid-min"},
		},
		{
			Name:       "Max Less Than Last Boundary",
			Count:      100,
			Sum:        5000,
			Min:        ptr(10.0),
			Max:        ptr(140.0), // Less than last boundary (150)
			Boundaries: []float64{25, 50, 75, 100, 150},
			Counts:     []uint64{20, 30, 25, 15, 8, 2}, // Has counts in overflow bucket
			Attributes: map[string]string{"service.name": "invalid-max"},
		},
		{
			Name:       "Sum Too Small",
			Count:      100,
			Sum:        100, // Too small given the boundaries and counts
			Min:        ptr(10.0),
			Max:        ptr(200.0),
			Boundaries: []float64{25, 50, 75, 100, 150},
			Counts:     []uint64{20, 30, 25, 15, 8, 2},
			Attributes: map[string]string{"service.name": "small-sum"},
		},
		{
			Name:       "Sum Too Large",
			Count:      100,
			Sum:        1000000, // Too large given the boundaries and counts
			Min:        ptr(10.0),
			Max:        ptr(200.0),
			Boundaries: []float64{25, 50, 75, 100, 150},
			Counts:     []uint64{20, 30, 25, 15, 8, 2},
			Attributes: map[string]string{"service.name": "large-sum"},
		},
		{
			Name:       "Only Counts No Boundaries",
			Count:      100,
			Sum:        5000,
			Min:        ptr(10.0),
			Max:        ptr(200.0),
			Boundaries: []float64{},
			Counts:     []uint64{100}, // Can't have counts without boundaries
			Attributes: map[string]string{"service.name": "counts-no-boundaries"},
		},
		{
			Name:       "Min in Second Bucket But Sum Too Low",
			Count:      100,
			Sum:        2000,      // This sum is too low given min is in second bucket
			Min:        ptr(60.0), // Min falls in second bucket (50,75]
			Max:        ptr(200.0),
			Boundaries: []float64{50, 75, 100, 150},
			Counts:     []uint64{20, 30, 25, 15, 10}, // 30 values must be at least 60 each in second bucket
			Attributes: map[string]string{"service.name": "invalid-min-bucket"},
		},
		{
			Name:       "Max in Second-to-Last Bucket But Sum Too High",
			Count:      100,
			Sum:        10000, // This sum is too high given max is in second-to-last bucket
			Min:        ptr(10.0),
			Max:        ptr(90.0), // Max falls in second-to-last bucket (75,100]
			Boundaries: []float64{50, 75, 100, 150},
			Counts:     []uint64{20, 30, 25, 15, 10}, // No value can exceed 90
			Attributes: map[string]string{"service.name": "invalid-max-bucket"},
		},
	}
}

func ptr(f float64) *float64 {
	return &f
}

func checkFeasibility(tc HistogramTestCase) (bool, string) {
	// Special case: empty histogram is valid
	if len(tc.Boundaries) == 0 && len(tc.Counts) == 0 {
		return true, ""
	}

	// Either both boundaries and counts must be present, or both must be empty
	if (len(tc.Boundaries) == 0) != (len(tc.Counts) == 0) {
		return false, fmt.Sprintf("boundaries and counts must either both be present or both be empty. boundaries: %d, counts: %d",
			len(tc.Boundaries), len(tc.Counts))
	}

	// Rest of checks only apply if we have boundaries/counts
	if len(tc.Boundaries) > 0 || len(tc.Counts) > 0 {
		// Check boundaries are in ascending order
		for i := 1; i < len(tc.Boundaries); i++ {
			if tc.Boundaries[i] <= tc.Boundaries[i-1] {
				return false, fmt.Sprintf("boundaries not in ascending order: %v <= %v",
					tc.Boundaries[i], tc.Boundaries[i-1])
			}
		}

		// Check counts array length
		if len(tc.Counts) != len(tc.Boundaries)+1 {
			return false, fmt.Sprintf("counts length (%d) should be boundaries length (%d) + 1",
				len(tc.Counts), len(tc.Boundaries))
		}

		// Verify total count matches sum of bucket counts
		var totalCount uint64
		for _, count := range tc.Counts {
			totalCount += count
		}
		if totalCount != tc.Count {
			return false, fmt.Sprintf("sum of counts (%d) doesn't match total count (%d)",
				totalCount, tc.Count)
		}

		// Check min/max feasibility if defined
		if tc.Min != nil {
			// If there are boundaries, first bucket must have counts > 0 only if min <= first boundary
			if len(tc.Boundaries) > 0 && tc.Counts[0] > 0 && *tc.Min > tc.Boundaries[0] {
				return false, fmt.Sprintf("min (%v) > first boundary (%v) but first bucket has counts",
					*tc.Min, tc.Boundaries[0])
			}
		}

		if tc.Max != nil {
			// If there are boundaries, last bucket must have counts > 0 only if max > last boundary
			if len(tc.Boundaries) > 0 && tc.Counts[len(tc.Counts)-1] > 0 &&
				*tc.Max <= tc.Boundaries[len(tc.Boundaries)-1] {
				return false, fmt.Sprintf("max (%v) <= last boundary (%v) but overflow bucket has counts",
					*tc.Max, tc.Boundaries[len(tc.Boundaries)-1])
			}
		}

		// Check sum feasibility
		if len(tc.Boundaries) > 0 {
			// Calculate minimum possible sum
			minSum := float64(0)
			if tc.Min != nil {
				// Find which bucket the minimum value belongs to
				minBucket := 0
				for i, bound := range tc.Boundaries {
					if *tc.Min > bound {
						minBucket = i + 1
					}
				}
				// Apply min value only from its containing bucket
				for i := minBucket; i < len(tc.Counts); i++ {
					if i == minBucket {
						minSum += float64(tc.Counts[i]) * *tc.Min
					} else {
						minSum += float64(tc.Counts[i]) * tc.Boundaries[i-1]
					}
				}
			} else {
				// Without min, use lower bounds
				for i := 1; i < len(tc.Counts); i++ {
					minSum += float64(tc.Counts[i]) * tc.Boundaries[i-1]
				}
			}

			// Calculate maximum possible sum
			maxSum := float64(0)
			if tc.Max != nil {
				// Find which bucket the maximum value belongs to
				maxBucket := len(tc.Boundaries) // Default to overflow bucket
				for i, bound := range tc.Boundaries {
					if *tc.Max <= bound {
						maxBucket = i
						break
					}
				}
				// Apply max value only up to its containing bucket
				for i := 0; i < len(tc.Counts); i++ {
					if i > maxBucket {
						maxSum += float64(tc.Counts[i]) * *tc.Max
					} else if i == len(tc.Boundaries) {
						maxSum += float64(tc.Counts[i]) * *tc.Max
					} else {
						maxSum += float64(tc.Counts[i]) * tc.Boundaries[i]
					}
				}
			} else {
				// If no max defined, we can't verify upper bound
				maxSum = math.Inf(1)
			}

			if tc.Sum < minSum {
				return false, fmt.Sprintf("sum (%v) is less than minimum possible sum (%v)",
					tc.Sum, minSum)
			}
			if maxSum != math.Inf(1) && tc.Sum > maxSum {
				return false, fmt.Sprintf("sum (%v) is greater than maximum possible sum (%v)",
					tc.Sum, maxSum)
			}
		}
	}

	return true, ""
}

func TestHistogramFeasibility(t *testing.T) {
	testCases := getTestCases()
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			feasible, reason := checkFeasibility(tc)
			assert.True(t, feasible, reason)
		})
	}
}

func TestInvalidHistogramFeasibility(t *testing.T) {
	invalidTestCases := getInvalidTestCases()

	for _, tc := range invalidTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			feasible, reason := checkFeasibility(tc)
			assert.False(t, feasible, reason)
		})
	}
}
