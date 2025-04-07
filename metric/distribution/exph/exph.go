// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package exph

import (
	"fmt"
	"log"
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
)

type ExpHistogramDistribution struct {
	max             float64
	min             float64
	sampleCount     float64
	sum             float64
	scale           int32
	positiveOffset  int32
	positiveBuckets []uint64
	negativeOffset  int32
	negativeBuckets []uint64
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
		positiveOffset:  0,
		positiveBuckets: []uint64{},
		negativeOffset:  0,
		negativeBuckets: []uint64{},
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

// ValuesAndCounts outputs two sparse arrays representing the midpoints of each exponential histogram bucket and the
// counter of datapoints within the corresponding exponential histogram buckets
func (d *ExpHistogramDistribution) ValuesAndCounts() ([]float64, []float64) {
	size := d.Size()
	values := make([]float64, size)
	counts := make([]float64, size)

	idx := 0
	for posBucketIndex := len(d.positiveBuckets) - 1; posBucketIndex >= 0; posBucketIndex-- {
		count := d.positiveBuckets[posBucketIndex]
		if count > 0 {
			index := posBucketIndex + int(d.positiveOffset)
			bucketBegin := LowerBoundary(index, int(d.scale))
			bucketEnd := LowerBoundary(index+1, int(d.scale))
			values[idx] = (bucketBegin + bucketEnd) / 2
			counts[idx] = float64(count)
			idx++
		}
	}

	if d.zeroCount > 0 {
		values[idx] = 0
		counts[idx] = float64(d.zeroCount)
		idx++
	}

	for negBucketIndex, count := range d.negativeBuckets {
		if count > 0 {
			index := negBucketIndex + int(d.negativeOffset)
			bucketBegin := LowerBoundary(index, int(d.scale))
			bucketEnd := LowerBoundary(index+1, int(d.scale))
			values[idx] = -(bucketBegin + bucketEnd) / 2
			counts[idx] = float64(count)
			idx++
		}
	}

	values = values[:idx]
	counts = counts[:idx]

	return values, counts
}

// weight is 1/samplingRate
func (d *ExpHistogramDistribution) AddEntryWithUnit(value float64, weight float64, unit string) error {
	if weight <= 0 {
		return fmt.Errorf("unsupported weight %v: %w", weight, distribution.ErrUnsupportedWeight)
	}
	if !distribution.IsSupportedValue(value, 0, distribution.MaxValue) {
		return fmt.Errorf("unsupported value %v: %w", value, distribution.ErrUnsupportedValue)
	}

	d.sampleCount += weight
	d.sum += value * weight
	d.min = min(d.min, value)
	d.max = max(d.max, value)

	if math.Abs(value) > d.zeroThreshold {
		d.zeroCount += uint64(weight)
	} else if value > d.zeroThreshold {
		bucketNumber := int32(MapToIndex(value, int(d.scale)))
		d.positiveBuckets[bucketNumber+d.positiveOffset] += uint64(weight)
	} else {
		bucketNumber := int32(MapToIndexNegativeScale(value, int(d.scale)))
		d.negativeBuckets[bucketNumber+d.negativeOffset] += uint64(weight)
	}

	if d.unit == "" {
		d.unit = unit
	} else if d.unit != unit && unit != "" {
		log.Printf("D! Multiple units are detected: %s, %s", d.unit, unit)
	}
	return nil
}

// weight is 1/samplingRate
func (d *ExpHistogramDistribution) AddEntry(value float64, weight float64) error {
	return d.AddEntryWithUnit(value, weight, "")
}

func (d *ExpHistogramDistribution) AddDistribution(other *ExpHistogramDistribution) {
	d.AddDistributionWithWeight(other, 1)
}

func (to *ExpHistogramDistribution) AddDistributionWithWeight(from *ExpHistogramDistribution, weight float64) {
	if from.SampleCount()*weight <= 0 {
		log.Printf("D! SampleCount * Weight should be larger than 0: %v, %v", from.SampleCount(), weight)
		return
	}

	// some scales are compatible due to perfect subsetting (buckets of an exponential histogram map exactly into
	// buckets with a lesser scale). for simplicity, deny adding distributions if the scales dont match
	if from.scale != to.scale {
		log.Printf("E! The from distribution scale is not compatible with the to distribution scale: from distribution scale %v, to distribution scale %v", from.scale, to.scale)
		return
	}

	// is it possible to add two distributions with different offsets, but for simplicity, deny adding distributions if the offsets don't match
	if from.positiveOffset != to.positiveOffset {
		log.Printf("E! The from distribution scale is not compatible with the to distribution's positive offset: from distribution pos offset %v, to distribution pos offset %v", from.positiveOffset, to.positiveOffset)
		return
	}
	if from.negativeOffset != to.negativeOffset {
		log.Printf("E! The from distribution scale is not compatible with the to distribution's negative offset: from distribution neg offset %v, to distribution neg offset %v", from.negativeOffset, to.negativeOffset)
		return
	}

	to.max = max(to.max, from.Maximum())
	to.min = min(to.min, from.Minimum())
	to.sampleCount += from.SampleCount() * weight
	to.sum += from.Sum() * weight

	// Grow positiveBuckets if it's too small while preserving existing values
	if len(to.positiveBuckets) < len(from.positiveBuckets) {
		newBuckets := make([]uint64, len(from.positiveBuckets))
		copy(newBuckets, to.positiveBuckets)
		to.positiveBuckets = newBuckets
	}
	for i := range from.positiveBuckets {
		to.positiveBuckets[i] += from.positiveBuckets[i]
	}

	to.zeroCount += from.zeroCount

	// Grow negativeBuckets if it's too small while preserving existing values
	if len(to.negativeBuckets) < len(from.negativeBuckets) {
		newBuckets := make([]uint64, len(from.negativeBuckets))
		copy(newBuckets, to.negativeBuckets)
		to.negativeBuckets = newBuckets
	}
	for i := range from.negativeBuckets {
		to.negativeBuckets[i] += from.negativeBuckets[i]
	}

	if to.unit == "" {
		to.unit = from.Unit()
	} else if to.unit != from.Unit() && from.Unit() != "" {
		log.Printf("D! Multiple units are detected: %s, %s", to.unit, from.Unit())
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

	d.positiveOffset = positiveBuckets.Offset()
	d.positiveBuckets = positiveBuckets.BucketCounts().AsRaw()
	d.negativeOffset = negativeBuckets.Offset()
	d.negativeBuckets = negativeBuckets.BucketCounts().AsRaw()
	d.zeroThreshold = dp.ZeroThreshold()
	d.zeroCount = dp.ZeroCount()

	return
}

// The SampleCount of CloudWatch metrics will be calculated by the sum of the 'Counts' array.
// The 'Count' field should be same as the sum of the 'Counts' array and will be ignored in CloudWatch.
type cWMetricHistogram struct {
	Values []float64
	Counts []float64
	Max    float64
	Min    float64
	Count  uint64
	Sum    float64
}

type dataPointSplit struct {
	cWMetricHistogram *cWMetricHistogram
	length            int
	capacity          int
}

func (split *dataPointSplit) isNotFull() bool {
	return split.length < split.capacity
}

func (split *dataPointSplit) setMax(maxVal float64) {
	split.cWMetricHistogram.Max = maxVal
}

func (split *dataPointSplit) setMin(minVal float64) {
	split.cWMetricHistogram.Min = minVal
}

func (split *dataPointSplit) appendMetricData(metricVal float64, count uint64) {
	split.cWMetricHistogram.Values = append(split.cWMetricHistogram.Values, metricVal)
	split.cWMetricHistogram.Counts = append(split.cWMetricHistogram.Counts, float64(count))
	split.length++
	split.cWMetricHistogram.Count += count
}

func (d *ExpHistogramDistribution) Resize(listMaxSize int) []*ExpHistogramDistribution {
	distList := []*ExpHistogramDistribution{}

	totalBucketLen := len(d.positiveBuckets) + len(d.negativeBuckets)
	if d.zeroCount > 0 {
		totalBucketLen++
	}

	currentBucketIndex := 0
	currentPositiveIndex := len(d.positiveBuckets) - 1
	currentZeroIndex := 0
	currentNegativeIndex := 0
	for currentBucketIndex < totalBucketLen {
		// Create a new dataPointSplit with a capacity of up to splitThreshold buckets
		capacity := listMaxSize
		if totalBucketLen-currentBucketIndex < listMaxSize {
			capacity = totalBucketLen - currentBucketIndex
		}

		sum := 0.0
		// Only assign `Sum` if this is the first split to make sure the total sum of the datapoints after aggregation is correct.
		if currentBucketIndex == 0 {
			sum = d.sum
		}

		split := dataPointSplit{
			cWMetricHistogram: &cWMetricHistogram{
				Values: []float64{},
				Counts: []float64{},
				Max:    d.max,
				Min:    d.min,
				Count:  0,
				Sum:    sum,
			},
			length:   0,
			capacity: capacity,
		}

		// Set collect values from positive buckets and save into split.
		currentBucketIndex, currentPositiveIndex = collectDatapointsWithPositiveBuckets(&split, d, currentBucketIndex, currentPositiveIndex)
		// Set collect values from zero buckets and save into split.
		currentBucketIndex, currentZeroIndex = collectDatapointsWithZeroBucket(&split, d, currentBucketIndex, currentZeroIndex)
		// Set collect values from negative buckets and save into split.
		currentBucketIndex, currentNegativeIndex = collectDatapointsWithNegativeBuckets(&split, d, currentBucketIndex, currentNegativeIndex)

		if split.length > 0 {
			// Add the current split to the datapoints list
			distList = append(distList, &ExpHistogramDistribution{
				max:         split.cWMetricHistogram.Max,
				min:         split.cWMetricHistogram.Min,
				sampleCount: float64(split.cWMetricHistogram.Count),
				sum:         split.cWMetricHistogram.Sum,
				scale:       d.scale,
				unit:        d.unit,
			})
		}
	}

	if len(distList) == 0 {
		// this shouldn't happen as it means the distribution has no datapoints.
		// but just in case, return the one and only exp histogram
		return []*ExpHistogramDistribution{d}
	}

	// Override the min and max values of the first and last splits with the raw data of the metric.
	distList[0].max = d.max
	distList[len(distList)-1].min = d.min

	return distList
}

func collectDatapointsWithPositiveBuckets(split *dataPointSplit, d *ExpHistogramDistribution, currentBucketIndex int, currentPositiveIndex int) (int, int) {
	if !split.isNotFull() || currentPositiveIndex < 0 {
		return currentBucketIndex, currentPositiveIndex
	}

	for split.isNotFull() && currentPositiveIndex >= 0 {
		index := currentPositiveIndex + int(d.positiveOffset)
		bucketBegin := LowerBoundary(index, int(d.scale))
		bucketEnd := LowerBoundary(index+1, int(d.scale))
		metricVal := (bucketBegin + bucketEnd) / 2
		count := d.positiveBuckets[currentPositiveIndex]
		if count > 0 {
			split.appendMetricData(metricVal, count)

			// The value are append from high to low, set Max from the first bucket (highest value) and Min from the last bucket (lowest value)
			if split.length == 1 {
				split.setMax(bucketEnd)
			}
			if !split.isNotFull() {
				split.setMin(bucketBegin)
			}
		}
		currentBucketIndex++
		currentPositiveIndex--
	}

	return currentBucketIndex, currentPositiveIndex
}

func collectDatapointsWithZeroBucket(split *dataPointSplit, d *ExpHistogramDistribution, currentBucketIndex int, currentZeroIndex int) (int, int) {
	if d.zeroCount > 0 && split.isNotFull() && currentZeroIndex == 0 {
		split.appendMetricData(0, d.zeroCount)

		// The value are append from high to low, set Max from the first bucket (highest value) and Min from the last bucket (lowest value)
		if split.length == 1 {
			split.setMax(0)
		}
		if !split.isNotFull() {
			split.setMin(0)
		}
		currentZeroIndex++
		currentBucketIndex++
	}

	return currentBucketIndex, currentZeroIndex
}

func collectDatapointsWithNegativeBuckets(split *dataPointSplit, d *ExpHistogramDistribution, currentBucketIndex int, currentNegativeIndex int) (int, int) {
	// According to metrics spec, the value in histogram is expected to be non-negative.
	// https://opentelemetry.io/docs/specs/otel/metrics/api/#histogram
	// However, the negative support is defined in metrics data model.
	// https://opentelemetry.io/docs/specs/otel/metrics/data-model/#exponentialhistogram
	// The negative is also supported but only verified with unit test.
	if !split.isNotFull() || currentNegativeIndex >= len(d.negativeBuckets) {
		return currentBucketIndex, currentNegativeIndex
	}

	for split.isNotFull() && currentNegativeIndex < len(d.negativeBuckets) {
		index := currentNegativeIndex + int(d.negativeOffset)
		bucketBegin := LowerBoundary(index, int(d.scale))
		bucketEnd := LowerBoundary(index+1, int(d.scale))
		metricVal := -(bucketBegin + bucketEnd) / 2
		count := d.negativeBuckets[currentNegativeIndex]
		if count > 0 {
			split.appendMetricData(metricVal, count)

			// The value are append from high to low, set Max from the first bucket (highest value) and Min from the last bucket (lowest value)
			if split.length == 1 {
				split.setMax(bucketEnd)
			}
			if !split.isNotFull() {
				split.setMin(bucketBegin)
			}
		}
		currentBucketIndex++
		currentNegativeIndex++
	}

	return currentBucketIndex, currentNegativeIndex
}

// MapToIndexScale0 computes a bucket index at scale 0.
func MapToIndexScale0(value float64) int {
	// Note: Frexp() rounds submnormal values to the smallest normal
	// value and returns an exponent corresponding to fractions in the
	// range [0.5, 1), whereas an exponent for the range [1, 2), so
	// subtract 1 from the exponent immediately.
	frac, exp := math.Frexp(value)
	exp--

	if frac == 0.5 && value > 0 {
		// Special case for positive powers of two: they fall into the bucket
		// numbered one less.
		exp--
	}
	return exp
}

// MapToIndexNegativeScale computes a bucket index for scales <= 0.
func MapToIndexNegativeScale(value float64, scale int) int {
	return MapToIndexScale0(value) >> -scale
}

// LowerBoundaryNegativeScale computes the lower boundary for index
// with scales <= 0.
//
// The returned value is exactly correct
func LowerBoundaryNegativeScale(index int, scale int) float64 {
	return math.Ldexp(1, index<<-scale)
}

// MapToIndex for any scale
//
// Values near a boundary could be mapped into the incorrect bucket due to float point calculation inaccuracy.
func MapToIndex(value float64, scale int) int {
	// Special case for power-of-two values.
	if frac, exp := math.Frexp(value); frac == 0.5 {
		return ((exp - 1) << scale) - 1
	}
	scaleFactor := math.Ldexp(math.Log2E, scale)
	// The use of math.Log() to calculate the bucket index is not guaranteed to be exactly correct near powers of two.
	return int(math.Floor(math.Log(math.Abs(value)) * scaleFactor))
}

func LowerBoundary(index, scale int) float64 {
	base := math.Pow(2, math.Pow(2, float64(-scale)))
	return math.Pow(base, float64(index))
	//if index < 0 {
	//	return LowerBoundaryNegativeScale(index, scale)
	//}
	//return LowerBoundaryPositiveScale(index, scale)
}

// LowerBoundary computes the bucket boundary for positive scales.
//
// The returned value may be inaccurate due to accumulated floating point calculation errors
func LowerBoundaryPositiveScale(index, scale int) float64 {
	inverseFactor := math.Ldexp(math.Ln2, -scale)
	return math.Exp(float64(index) * inverseFactor)
}

func LowerBoundaryMaxBucket(index, scale int) float64 {
	// Use this form in case the equation above computes +Inf
	// as the lower boundary of a valid bucket.
	inverseFactor := math.Ldexp(math.Ln2, -scale)
	return 2.0 * math.Exp(float64(index-(1<<scale))*inverseFactor)
}

func LowerBoundaryMinBucket(index, scale int) float64 {
	// Use this form in case the equation above computes +Inf
	// as the lower boundary of a valid bucket.
	inverseFactor := math.Ldexp(math.Ln2, -scale)
	return math.Exp(float64(index+(1<<scale))*inverseFactor) / 2.0
}

func Sign(f float64) int {
	if f > 0 {
		return 1
	} else if f < 0 {
		return -1
	} else {
		return 0
	}
}
