// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package exph

import (
	"cmp"
	"log"
	"maps"
	"math"
	"slices"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

type ExpHistogramDistribution struct {
	max             float64
	min             float64
	sampleCount     float64
	sum             float64
	scale           int32
	positiveBuckets map[int]uint64 // map of bucket index to count
	negativeBuckets map[int]uint64 // map of bucket index to count
	zeroThreshold   float64
	zeroCount       uint64
	unit            string
}

func NewExpHistogramDistribution() *ExpHistogramDistribution {
	return &ExpHistogramDistribution{
		max:             -math.MaxFloat64,
		min:             math.MaxFloat64,
		sampleCount:     0,
		sum:             0,
		scale:           0,
		unit:            "",
		positiveBuckets: map[int]uint64{},
		negativeBuckets: map[int]uint64{},
		zeroThreshold:   0,
		zeroCount:       0,
	}
}

func (d *ExpHistogramDistribution) Maximum() float64 {
	return d.max
}

func (d *ExpHistogramDistribution) Minimum() float64 {
	return d.min
}

func (d *ExpHistogramDistribution) SampleCount() float64 {
	return d.sampleCount
}

func (d *ExpHistogramDistribution) Sum() float64 {
	return d.sum
}

func (d *ExpHistogramDistribution) Unit() string {
	return d.unit
}

func (d *ExpHistogramDistribution) Size() int {
	size := len(d.negativeBuckets) + len(d.positiveBuckets)
	if d.zeroCount > 0 {
		size++
	}
	return size
}

// ValuesAndCounts outputs two arrays representing the midpoints of each exponential histogram bucket and the
// counter of datapoints within the corresponding exponential histogram buckets
func (d *ExpHistogramDistribution) ValuesAndCounts() ([]float64, []float64) {
	values := []float64{}
	counts := []float64{}

	// iterate through positive buckets in descending order
	posOffsetIndicies := slices.SortedFunc(maps.Keys(d.positiveBuckets), func(a, b int) int {
		return cmp.Compare(b, a)
	})
	for _, offsetIndex := range posOffsetIndicies {
		counter := d.positiveBuckets[offsetIndex]
		bucketBegin := LowerBoundary(offsetIndex, int(d.scale))
		bucketEnd := LowerBoundary(offsetIndex+1, int(d.scale))
		value := (bucketBegin + bucketEnd) / 2.0
		values = append(values, value)
		counts = append(counts, float64(counter))
	}

	if d.zeroCount > 0 {
		values = append(values, 0)
		counts = append(counts, float64(d.zeroCount))
	}

	// iterate through negative buckets in ascending order so that the values array is entirely descending
	negOffsetIndicies := slices.Sorted(maps.Keys(d.negativeBuckets))
	for _, offsetIndex := range negOffsetIndicies {
		counter := d.negativeBuckets[offsetIndex]
		bucketBegin := LowerBoundary(offsetIndex, int(d.scale))
		bucketEnd := LowerBoundary(offsetIndex+1, int(d.scale))
		value := -(bucketBegin + bucketEnd) / 2.0
		values = append(values, value)
		counts = append(counts, float64(counter))
	}

	return values, counts
}

func (d *ExpHistogramDistribution) AddDistribution(from *ExpHistogramDistribution) {
	if from.SampleCount() <= 0 {
		log.Printf("D! SampleCount should be larger than 0: %v", from.SampleCount())
		return
	}

	// some scales are compatible due to perfect subsetting (buckets of an exponential histogram map exactly into
	// buckets with a lesser scale). for simplicity, deny adding distributions if the scales dont match
	if from.scale != d.scale {
		log.Printf("E! The from distribution scale is not compatible with the to distribution scale: from distribution scale %v, to distribution scale %v", from.scale, d.scale)
		return
	}

	if from.zeroThreshold != d.zeroThreshold {
		log.Printf("E! The from distribution zeroThreshold is not compatible with the to distribution zeroThreshold: from distribution zeroThreshold %v, to distribution zeroThreshold %v", from.zeroThreshold, d.zeroThreshold)
		return
	}

	d.max = max(d.max, from.Maximum())
	d.min = min(d.min, from.Minimum())
	d.sampleCount += from.SampleCount()
	d.sum += from.Sum()

	for i := range from.positiveBuckets {
		d.positiveBuckets[i] += from.positiveBuckets[i]
	}

	d.zeroCount += from.zeroCount

	for i := range from.negativeBuckets {
		d.negativeBuckets[i] += from.negativeBuckets[i]
	}

	if d.unit == "" {
		d.unit = from.Unit()
	} else if d.unit != from.Unit() && from.Unit() != "" {
		log.Printf("D! Multiple units are detected: %s, %s", d.unit, from.Unit())
	}

}

func (d *ExpHistogramDistribution) ConvertFromOtel(dp pmetric.ExponentialHistogramDataPoint, unit string) {
	positiveBuckets := dp.Positive()
	negativeBuckets := dp.Negative()

	d.scale = dp.Scale()
	d.unit = unit

	d.max = dp.Max()
	d.min = dp.Min()
	d.sampleCount = float64(dp.Count())
	d.sum = dp.Sum()

	positiveOffset := positiveBuckets.Offset()
	posBucketCounts := positiveBuckets.BucketCounts().AsRaw()
	for posBucketIndex := range posBucketCounts {
		offsetIndex := posBucketIndex + int(positiveOffset)
		d.positiveBuckets[offsetIndex] = posBucketCounts[posBucketIndex]
	}

	d.zeroThreshold = dp.ZeroThreshold()
	d.zeroCount = dp.ZeroCount()

	negativeOffset := negativeBuckets.Offset()
	negBucketCounts := negativeBuckets.BucketCounts().AsRaw()
	for negBucketIndex := range negBucketCounts {
		offsetIndex := negBucketIndex + int(negativeOffset)
		d.negativeBuckets[offsetIndex] = negBucketCounts[negBucketIndex]
	}
}

func (d *ExpHistogramDistribution) Resize(_ int) []*ExpHistogramDistribution {
	// TODO: split data points into separate PMD requests if the number of buckets exceeds the API limit
	return []*ExpHistogramDistribution{d}
}
