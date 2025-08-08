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

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
)

type ExpHistogramDistribution struct {
	max             float64
	min             float64
	sampleCount     float64
	sum             float64
	scale           int
	positiveBuckets map[int]uint64 // map of bucket index to count
	negativeBuckets map[int]uint64 // map of bucket index to count
	zeroThreshold   float64
	zeroCount       uint64
	unit            string
}

func NewExponentialDistribution() distribution.ExponentialDistribution {
	return newExpHistogramDistribution()
}

func newExpHistogramDistribution() *ExpHistogramDistribution {
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
		bucketBegin := LowerBoundary(offsetIndex, d.scale)
		bucketEnd := LowerBoundary(offsetIndex+1, d.scale)
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
		bucketBegin := LowerBoundary(offsetIndex, d.scale)
		bucketEnd := LowerBoundary(offsetIndex+1, d.scale)
		value := -(bucketBegin + bucketEnd) / 2.0
		values = append(values, value)
		counts = append(counts, float64(counter))
	}

	return values, counts
}

// weight is 1/samplingRate
func (d *ExpHistogramDistribution) AddEntry(value float64, weight float64) error {
	return d.AddEntryWithUnit(value, weight, "")
}

// weight is 1/samplingRate
func (d *ExpHistogramDistribution) AddEntryWithUnit(value float64, weight float64, unit string) error {
	return nil
}

func (d *ExpHistogramDistribution) AddDistribution(from distribution.Distribution) {

	expFrom, ok := from.(*ExpHistogramDistribution)
	if !ok {
		log.Printf("E! The from distribution is not an exponential histogram distribution. Cannot add distributions: %v", from)
		return
	}

	if from.SampleCount() <= 0 {
		log.Printf("D! SampleCount should be larger than 0: %v", from.SampleCount())
		return
	}

	// some scales are compatible due to perfect subsetting (buckets of an exponential histogram map exactly into
	// buckets with a lesser scale). for simplicity, deny adding distributions if the scales dont match
	if expFrom.scale != d.scale {
		log.Printf("E! The from distribution scale is not compatible with the to distribution scale: from distribution scale %v, to distribution scale %v", expFrom.scale, d.scale)
		return
	}

	if expFrom.zeroThreshold != d.zeroThreshold {
		log.Printf("E! The from distribution zeroThreshold is not compatible with the to distribution zeroThreshold: from distribution zeroThreshold %v, to distribution zeroThreshold %v", expFrom.zeroThreshold, d.zeroThreshold)
		return
	}

	d.max = max(d.max, expFrom.Maximum())
	d.min = min(d.min, expFrom.Minimum())
	d.sampleCount += expFrom.SampleCount()
	d.sum += expFrom.Sum()

	for i := range expFrom.positiveBuckets {
		d.positiveBuckets[i] += expFrom.positiveBuckets[i]
	}

	d.zeroCount += expFrom.zeroCount

	for i := range expFrom.negativeBuckets {
		d.negativeBuckets[i] += expFrom.negativeBuckets[i]
	}

	if d.unit == "" {
		d.unit = expFrom.Unit()
	} else if d.unit != expFrom.Unit() && expFrom.Unit() != "" {
		log.Printf("D! Multiple units are detected: %s, %s", d.unit, expFrom.Unit())
	}

}

func (d *ExpHistogramDistribution) ConvertFromOtel(dp pmetric.ExponentialHistogramDataPoint, unit string) {
	positiveBuckets := dp.Positive()
	negativeBuckets := dp.Negative()

	d.scale = int(dp.Scale())
	d.unit = unit

	d.max = dp.Max()
	d.min = dp.Min()
	d.sampleCount = float64(dp.Count())
	d.sum = dp.Sum()

	// Each range of the ExponentialHistogram data point uses a dense representation of the buckets, where a range of buckets
	// is expressed as a single `offset` value, a signed integer, and an array of count values, where array element i
	// represents the bucket count for bucket index offset+i.
	positiveOffset := positiveBuckets.Offset()
	posBucketCounts := positiveBuckets.BucketCounts().AsRaw()
	for i := range posBucketCounts {
		offsetIndex := i + int(positiveOffset)
		d.positiveBuckets[offsetIndex] = posBucketCounts[i]
	}

	d.zeroThreshold = dp.ZeroThreshold()
	d.zeroCount = dp.ZeroCount()

	negativeOffset := negativeBuckets.Offset()
	negBucketCounts := negativeBuckets.BucketCounts().AsRaw()
	for i := range negBucketCounts {
		offsetIndex := i + int(negativeOffset)
		d.negativeBuckets[offsetIndex] = negBucketCounts[i]
	}
}

func (d *ExpHistogramDistribution) Resize(_ int) []distribution.Distribution {
	// TODO: split data points into separate PMD requests if the number of buckets exceeds the API limit
	return []distribution.Distribution{d}
}
