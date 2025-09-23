// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package regular

import (
	"fmt"
	"log"
	"maps"
	"math"
	"slices"
	"sort"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/seh1"
)

type RegularDistribution struct {
	maximum     float64
	minimum     float64
	sampleCount float64
	sum         float64
	buckets     map[float64]float64 // from  value to the counter (i.e. weight)
	unit        string
}

func NewRegularDistribution() distribution.ClassicDistribution {
	return &RegularDistribution{
		maximum:     0, // negative number is not supported for now, so zero is the min value
		minimum:     math.MaxFloat64,
		sampleCount: 0,
		sum:         0,
		buckets:     map[float64]float64{},
		unit:        "",
	}
}

func NewFromOtelOriginal(dp pmetric.HistogramDataPoint) ToCloudWatchValuesAndCounts {
	rd := &RegularDistribution{
		maximum:     0, // negative number is not supported for now, so zero is the min value
		minimum:     math.MaxFloat64,
		sampleCount: 0,
		sum:         0,
		buckets:     map[float64]float64{},
		unit:        "",
	}
	rd.maximum = dp.Max()
	rd.minimum = dp.Min()
	rd.sampleCount = float64(dp.Count())
	rd.sum = dp.Sum()
	// This is incorrect as ExplicitBounds defined boundaries between buckets
	//  len(dp.BucketCounts) = len(dp.ExplicitBounds) + 1
	// This algorithm misses the last bucket count
	for i := 0; i < dp.ExplicitBounds().Len(); i++ {
		k := dp.ExplicitBounds().At(i)
		v := dp.BucketCounts().At(i)
		rd.buckets[k] = float64(v)
	}
	return rd
}

func (regularDist *RegularDistribution) Maximum() float64 {
	return regularDist.maximum
}

func (regularDist *RegularDistribution) Minimum() float64 {
	return regularDist.minimum
}

func (regularDist *RegularDistribution) SampleCount() float64 {
	return regularDist.sampleCount
}

func (regularDist *RegularDistribution) Sum() float64 {
	return regularDist.sum
}

func (regularDist *RegularDistribution) ValuesAndCounts() (values []float64, counts []float64) {
	values = []float64{}
	counts = []float64{}
	keys := slices.Collect(maps.Keys(regularDist.buckets))
	slices.Sort(keys)
	for _, value := range keys {
		counter := regularDist.buckets[value]
		values = append(values, value)
		counts = append(counts, counter)
	}
	return
}

func (regularDist *RegularDistribution) Unit() string {
	return regularDist.unit
}

func (regularDist *RegularDistribution) Size() int {
	return len(regularDist.buckets)
}

// weight is 1/samplingRate
func (regularDist *RegularDistribution) AddEntryWithUnit(value float64, weight float64, unit string) error {
	if weight <= 0 {
		return fmt.Errorf("unsupported weight %v: %w", weight, distribution.ErrUnsupportedWeight)
	}
	if !distribution.IsSupportedValue(value, 0, distribution.MaxValue) {
		return fmt.Errorf("unsupported value %v: %w", value, distribution.ErrUnsupportedValue)
	}
	//sample count
	regularDist.sampleCount += weight
	//sum
	regularDist.sum += value * weight
	//min
	if value < regularDist.minimum {
		regularDist.minimum = value
	}
	//max
	if value > regularDist.maximum {
		regularDist.maximum = value
	}

	//values and counts
	regularDist.buckets[value] += weight

	//unit
	if regularDist.unit == "" {
		regularDist.unit = unit
	} else if regularDist.unit != unit && unit != "" {
		log.Printf("D! Multiple units are detected: %s, %s", regularDist.unit, unit)
	}
	return nil
}

// weight is 1/samplingRate
func (regularDist *RegularDistribution) AddEntry(value float64, weight float64) error {
	return regularDist.AddEntryWithUnit(value, weight, "")
}

func (regularDist *RegularDistribution) AddDistribution(distribution distribution.Distribution) {
	regularDist.AddDistributionWithWeight(distribution, 1)
}

func (regularDist *RegularDistribution) AddDistributionWithWeight(distribution distribution.Distribution, weight float64) {
	if distribution.SampleCount()*weight > 0 {

		//values and counts
		if fromDistribution, ok := distribution.(*RegularDistribution); ok {
			for bucketNumber, bucketCounts := range fromDistribution.buckets {
				regularDist.buckets[bucketNumber] += bucketCounts * weight
			}
		} else {
			log.Printf("E! The from distribution type is not compatible with the to distribution type: from distribution type %T, to distribution type %T", regularDist, distribution)
			return
		}

		//sample count
		regularDist.sampleCount += distribution.SampleCount() * weight
		//sum
		regularDist.sum += distribution.Sum() * weight
		//min
		if distribution.Minimum() < regularDist.minimum {
			regularDist.minimum = distribution.Minimum()
		}
		//max
		if distribution.Maximum() > regularDist.maximum {
			regularDist.maximum = distribution.Maximum()
		}

		//unit
		if regularDist.unit == "" {
			regularDist.unit = distribution.Unit()
		} else if regularDist.unit != distribution.Unit() && distribution.Unit() != "" {
			log.Printf("D! Multiple units are dected: %s, %s", regularDist.unit, distribution.Unit())
		}
	} else {
		log.Printf("D! SampleCount * Weight should be larger than 0: %v, %v", distribution.SampleCount(), weight)
	}
}

func (rd *RegularDistribution) ConvertToOtel(dp pmetric.HistogramDataPoint) {
	dp.SetMax(rd.maximum)
	dp.SetMin(rd.minimum)
	dp.SetCount(uint64(rd.sampleCount))
	dp.SetSum(rd.sum)
	dp.ExplicitBounds().EnsureCapacity(len(rd.buckets))
	dp.BucketCounts().EnsureCapacity(len(rd.buckets))
	for k, v := range rd.buckets {
		dp.ExplicitBounds().Append(k)
		// Beware of potential loss of precision due to type conversion.
		dp.BucketCounts().Append(uint64(v))
	}
}

func (rd *RegularDistribution) ConvertFromOtel(dp pmetric.HistogramDataPoint, unit string) {
	rd.maximum = dp.Max()
	rd.minimum = dp.Min()
	rd.sampleCount = float64(dp.Count())
	rd.sum = dp.Sum()
	rd.unit = unit
	for i := 0; i < dp.ExplicitBounds().Len(); i++ {
		k := dp.ExplicitBounds().At(i)
		v := dp.BucketCounts().At(i)
		rd.buckets[k] = float64(v)
	}
}

func (regularDist *RegularDistribution) Resize(listMaxSize int) []distribution.Distribution {
	distList := []distribution.Distribution{}
	values, _ := regularDist.ValuesAndCounts()
	sort.Float64s(values)
	newSEH1Dist := seh1.NewSEH1Distribution().(*seh1.SEH1Distribution)
	for i := 0; i < len(values); i++ {
		if !newSEH1Dist.CanAdd(values[i], listMaxSize) {
			distList = append(distList, newSEH1Dist)
			newSEH1Dist = seh1.NewSEH1Distribution().(*seh1.SEH1Distribution)
		}
		newSEH1Dist.AddEntry(values[i], regularDist.GetCount(values[i]))
	}
	if newSEH1Dist.Size() > 0 {
		distList = append(distList, newSEH1Dist)
	}
	return distList
}

func (regularDist *RegularDistribution) GetCount(value float64) float64 {
	return regularDist.buckets[value]
}

var ErrUnsupportedOperation = fmt.Errorf("unsupported operation")

type ToCloudWatchValuesAndCounts interface {
	ValuesAndCounts() ([]float64, []float64)
	Sum() float64
	SampleCount() float64
	Minimum() float64
	Maximum() float64
}

type MidpointMapping struct {
	maximum     float64
	minimum     float64
	sampleCount float64
	sum         float64
	values      []float64
	counts      []float64
}

type EvenMapping struct {
	maximum     float64
	minimum     float64
	sampleCount float64
	sum         float64
	values      []float64
	counts      []float64
}

type ExponentialMapping struct {
	maximum     float64
	minimum     float64
	sampleCount float64
	sum         float64
	values      []float64
	counts      []float64
}
type ExponentialMappingCW struct {
	maximum     float64
	minimum     float64
	sampleCount float64
	sum         float64
	values      []float64
	counts      []float64
}

var _ (ToCloudWatchValuesAndCounts) = (*MidpointMapping)(nil)
var _ (ToCloudWatchValuesAndCounts) = (*EvenMapping)(nil)
var _ (ToCloudWatchValuesAndCounts) = (*ExponentialMapping)(nil)
var _ (ToCloudWatchValuesAndCounts) = (*ExponentialMappingCW)(nil)

func NewEvenMappingFromOtel(dp pmetric.HistogramDataPoint) ToCloudWatchValuesAndCounts {
	em := &EvenMapping{
		maximum:     dp.Max(),
		minimum:     dp.Min(),
		sampleCount: float64(dp.Count()),
		sum:         dp.Sum(),
	}

	bounds := dp.ExplicitBounds()
	bucketCounts := dp.BucketCounts()
	values := make([]float64, 0)
	counts := make([]float64, 0)
	maxInnerBucketCount := 50

	for i := 0; i < bounds.Len()-1; i++ {
		sampleCount := float64(bucketCounts.At(i))
		if sampleCount > 0 {
			innerBucketCount := int(math.Min(sampleCount, float64(maxInnerBucketCount)))
			delta := (bounds.At(i+1) - bounds.At(i)) / float64(innerBucketCount)
			valueSampleCount := sampleCount / float64(innerBucketCount)

			for n := 0; n < innerBucketCount; n++ {
				value := bounds.At(i) + delta*float64(n)
				values = append(values, value)
				counts = append(counts, valueSampleCount)
			}
		}
	}

	em.values = values
	em.counts = counts
	return em
}

func (em *EvenMapping) ValuesAndCounts() ([]float64, []float64) {
	return em.values, em.counts
}

func (em *EvenMapping) Minimum() float64 {
	return em.minimum
}

func (em *EvenMapping) Maximum() float64 {
	return em.maximum
}

func (em *EvenMapping) SampleCount() float64 {
	return em.sampleCount
}

func (em *EvenMapping) Sum() float64 {
	return em.sum
}

func NewMidpointMappingFromOtel(dp pmetric.HistogramDataPoint) ToCloudWatchValuesAndCounts {
	mm := &MidpointMapping{
		maximum:     dp.Max(),
		minimum:     dp.Min(),
		sampleCount: float64(dp.Count()),
		sum:         dp.Sum(),
	}

	bounds := dp.ExplicitBounds()
	bucketCounts := dp.BucketCounts()
	values := make([]float64, 0)
	counts := make([]float64, 0)

	for i := 0; i < bounds.Len()-1; i++ {
		if bucketCounts.At(i) > 0 {
			midpoint := (bounds.At(i) + bounds.At(i+1)) / 2
			values = append(values, midpoint)
			counts = append(counts, float64(bucketCounts.At(i)))
		}
	}

	mm.values = values
	mm.counts = counts
	return mm
}

func (mm *MidpointMapping) ValuesAndCounts() ([]float64, []float64) {
	return mm.values, mm.counts
}

func (mm *MidpointMapping) Minimum() float64 {
	return mm.minimum
}

func (mm *MidpointMapping) Maximum() float64 {
	return mm.maximum
}

func (mm *MidpointMapping) SampleCount() float64 {
	return mm.sampleCount
}

func (mm *MidpointMapping) Sum() float64 {
	return mm.sum
}

func NewExponentialMappingFromOtel(dp pmetric.HistogramDataPoint) ToCloudWatchValuesAndCounts {
	em := &ExponentialMappingCW{
		maximum:     0,
		minimum:     math.MaxFloat64,
		sampleCount: float64(dp.Count()),
		sum:         dp.Sum(),
		values:      make([]float64, 0),
		counts:      make([]float64, 0),
	}

	bounds := dp.ExplicitBounds()
	bucketCounts := dp.BucketCounts()

	// No validations - assuming valid input histogram

	em.minimum = dp.Min()
	em.maximum = dp.Max()
	naturalMapping := make(map[float64]float64)

	for i := 0; i < bucketCounts.Len(); i++ {
		sampleCount := bucketCounts.At(i)
		if sampleCount == 0 {
			continue
		}

		// Determine bucket bounds
		var lowerBound, upperBound float64
		if i == 0 {
			lowerBound = em.minimum
		} else {
			lowerBound = bounds.At(i - 1)
		}

		if i == bucketCounts.Len()-1 {
			upperBound = em.maximum
		} else {
			upperBound = bounds.At(i)
		}

		// Calculate magnitude for next bucket comparison
		magnitude := -1.0
		if i < bucketCounts.Len()-1 {
			nextSampleCount := bucketCounts.At(i + 1)
			var nextUpperBound float64
			if i+1 == bucketCounts.Len()-1 {
				nextUpperBound = em.maximum
			} else {
				nextUpperBound = bounds.At(i + 1)
			}
			magnitude = math.Log(((upperBound - lowerBound) / float64(sampleCount)) / ((nextUpperBound - upperBound) / float64(nextSampleCount)))
		}

		innerBucketCount := int(min(sampleCount, 50))
		delta := (upperBound - lowerBound) / float64(innerBucketCount)
		innerHistogram := make(map[float64]float64)

		if magnitude < 0 { // Use -yx^2
			sigma := float64(innerBucketCount*(innerBucketCount+1)*(2*innerBucketCount+1)) / 6.0 // closed form of sum(x^2, 0, innerBucketCount)
			epsilon := float64(sampleCount) / sigma

			for j := 0; j < innerBucketCount; j++ {
				innerBucketSampleCount := epsilon * math.Pow(float64(j-innerBucketCount), 2.0)
				if j%2 == 0 {
					innerBucketSampleCount = math.Ceil(innerBucketSampleCount)
				} else {
					innerBucketSampleCount = math.Floor(innerBucketSampleCount)
				}

				if innerBucketSampleCount > 0 {
					innerHistogram[lowerBound+delta*float64(j+1)] = innerBucketSampleCount
				}
			}
		} else if magnitude < 1 { // Use x
			for j := 1; j <= innerBucketCount; j++ {
				innerHistogram[lowerBound+delta*float64(j)] = float64(sampleCount) / float64(innerBucketCount)
			}
		} else { // Use yx^2
			sigma := float64(innerBucketCount*(innerBucketCount+1)*(2*innerBucketCount+1)) / 6.0 // closed form of sum(x^2, 0, innerBucketCount)
			epsilon := float64(sampleCount) / sigma

			for j := 0; j < innerBucketCount; j++ {
				innerBucketSampleCount := epsilon * math.Pow(float64(j), 2.0)
				if j%2 == 0 {
					innerBucketSampleCount = math.Ceil(innerBucketSampleCount)
				} else {
					innerBucketSampleCount = math.Floor(innerBucketSampleCount)
				}

				if innerBucketSampleCount > 0 {
					innerHistogram[lowerBound+delta*float64(j+1)] = innerBucketSampleCount
				}
			}
		}

		for k, v := range innerHistogram {
			naturalMapping[k] = v
		}
	}

	// Move last entry to histogramMax
	if len(naturalMapping) > 0 {
		var lastKey float64
		var lastValue float64
		for k, v := range naturalMapping {
			if k > lastKey {
				lastKey = k
				lastValue = v
			}
		}
		delete(naturalMapping, lastKey)
		naturalMapping[em.maximum] = lastValue
	}

	keys := slices.Collect(maps.Keys(naturalMapping))
	slices.Sort(keys)
	em.values = make([]float64, len(keys))
	em.counts = make([]float64, len(keys))
	for i, k := range keys {
		em.values[i] = k
		em.counts[i] = naturalMapping[k]
	}

	return em
}

func (em *ExponentialMapping) ValuesAndCounts() ([]float64, []float64) {
	return em.values, em.counts
}

func (em *ExponentialMapping) Minimum() float64 {
	return em.minimum
}

func (em *ExponentialMapping) Maximum() float64 {
	return em.maximum
}

func (em *ExponentialMapping) SampleCount() float64 {
	return em.sampleCount
}

func (em *ExponentialMapping) Sum() float64 {
	return em.sum
}

func NewExponentialMappingCWFromOtel(dp pmetric.HistogramDataPoint) ToCloudWatchValuesAndCounts {
	em := &ExponentialMappingCW{
		maximum:     0,
		minimum:     math.MaxFloat64,
		sampleCount: float64(dp.Count()),
		sum:         dp.Sum(),
		values:      make([]float64, 0),
		counts:      make([]float64, 0),
	}

	bounds := dp.ExplicitBounds()
	bucketCounts := dp.BucketCounts()

	// No validations - assuming valid input histogram

	em.minimum = dp.Min()
	if !dp.HasMin() {
		// No minimum implies open lower boundary on first bucket
		// Cap lower bound to top end of first bucket.
		if bounds.Len() > 0 {
			em.minimum = bounds.At(0)
		} else {
			em.minimum = math.Inf(-1)
		}
	}

	em.maximum = dp.Max()
	if !dp.HasMax() {
		// No maximum implies open upper boundary on last bucket
		// Cap upper bound to lower end of last bucket.
		if bounds.Len() > 0 {
			em.maximum = bounds.At(bounds.Len() - 1)
		} else {
			em.maximum = math.Inf(1)
		}

	}

	// Special case: no boundaries implies a single bucket
	if bounds.Len() == 0 {
		em.counts = append(em.counts, float64(bucketCounts.At(0)))
		// if min and max aren't defined, then this is a useless measure.
		if dp.HasMax() && dp.HasMin() {
			em.values = append(em.values, dp.Min()+(dp.Max()-dp.Min())/2.0)
		} else if dp.HasMax() {
			em.values = append(em.values, dp.Max())
		} else if dp.HasMin() {
			em.values = append(em.values, dp.Max())
		} else {
			em.values = append(em.values, 0) // arbitrary value
		}
		return em
	}

	naturalMapping := make(map[float64]float64)

	for i := 0; i < bucketCounts.Len(); i++ {
		sampleCount := bucketCounts.At(i)
		if sampleCount == 0 {
			continue
		}

		// Determine bucket bounds
		var lowerBound, upperBound float64
		if i == 0 {
			lowerBound = em.minimum
		} else {
			lowerBound = bounds.At(i - 1)
		}

		if i == bucketCounts.Len()-1 {
			upperBound = em.maximum
		} else {
			upperBound = bounds.At(i)
		}

		// Calculate magnitude for next bucket comparison
		magnitude := -1.0
		if i < bucketCounts.Len()-1 {
			nextSampleCount := bucketCounts.At(i + 1)
			var nextUpperBound float64
			if i+1 == bucketCounts.Len()-1 {
				nextUpperBound = em.maximum
			} else {
				nextUpperBound = bounds.At(i + 1)
			}
			magnitude = math.Log(((upperBound - lowerBound) / float64(sampleCount)) / ((nextUpperBound - upperBound) / float64(nextSampleCount)))
		}

		innerBucketCount := int(min(sampleCount, 50))
		delta := (upperBound - lowerBound) / float64(innerBucketCount)
		innerHistogram := make(map[float64]float64)

		if magnitude < 0 { // Use -yx^2
			sigma := float64(innerBucketCount) * float64(innerBucketCount+1) * float64(2*innerBucketCount+1) / 6.0
			epsilon := float64(sampleCount) / sigma

			for j := 0; j < innerBucketCount; j++ {
				innerBucketSampleCount := epsilon * math.Pow(float64(j-innerBucketCount), 2.0)
				if j%2 == 0 {
					innerBucketSampleCount = math.Ceil(innerBucketSampleCount)
				} else {
					innerBucketSampleCount = math.Floor(innerBucketSampleCount)
				}

				if innerBucketSampleCount > 0 {
					innerHistogram[lowerBound+delta*float64(j+1)] = innerBucketSampleCount
				}
			}
		} else if magnitude < 1 { // Use x
			for j := 1; j <= innerBucketCount; j++ {
				innerHistogram[lowerBound+delta*float64(j)] = float64(sampleCount) / float64(innerBucketCount)
			}
		} else { // Use yx^2
			sigma := float64(innerBucketCount) * float64(innerBucketCount+1) * float64(2*innerBucketCount+1) / 6.0
			epsilon := float64(sampleCount) / sigma

			for j := 0; j < innerBucketCount; j++ {
				innerBucketSampleCount := epsilon * math.Pow(float64(j), 2.0)
				if j%2 == 0 {
					innerBucketSampleCount = math.Ceil(innerBucketSampleCount)
				} else {
					innerBucketSampleCount = math.Floor(innerBucketSampleCount)
				}

				if innerBucketSampleCount > 0 {
					innerHistogram[lowerBound+delta*float64(j+1)] = innerBucketSampleCount
				}
			}
		}

		for k, v := range innerHistogram {
			naturalMapping[k] = v
		}
	}

	// Move last entry to histogramMax
	if len(naturalMapping) > 0 {
		var lastKey float64
		var lastValue float64
		for k, v := range naturalMapping {
			if k > lastKey {
				lastKey = k
				lastValue = v
			}
		}
		delete(naturalMapping, lastKey)
		naturalMapping[em.maximum] = lastValue
	}

	keys := slices.Collect(maps.Keys(naturalMapping))
	slices.Sort(keys)
	em.values = make([]float64, len(keys))
	em.counts = make([]float64, len(keys))
	for i, k := range keys {
		em.values[i] = k
		em.counts[i] = naturalMapping[k]
	}

	return em
}

func (em *ExponentialMappingCW) ValuesAndCounts() ([]float64, []float64) {
	return em.values, em.counts
}

func (em *ExponentialMappingCW) Minimum() float64 {
	return em.minimum
}

func (em *ExponentialMappingCW) Maximum() float64 {
	return em.maximum
}

func (em *ExponentialMappingCW) SampleCount() float64 {
	return em.sampleCount
}

func (em *ExponentialMappingCW) Sum() float64 {
	return em.sum
}
