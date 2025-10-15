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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/aws/cloudwatch/histograms"
)

type RegularDistribution2 struct {
	dp   pmetric.HistogramDataPoint
	unit string
}

func (regularDist *RegularDistribution2) Maximum() float64 {
	return regularDist.dp.Max()
}

func (regularDist *RegularDistribution2) Minimum() float64 {
	return regularDist.dp.Min()
}

func (regularDist *RegularDistribution2) SampleCount() float64 {
	return float64(regularDist.dp.Count())
}

func (regularDist *RegularDistribution2) Sum() float64 {
	return regularDist.dp.Sum()
}

func (regularDist *RegularDistribution2) ValuesAndCounts() (values []float64, counts []float64) {
	return histograms.ConvertOTelToCloudWatch(regularDist.dp).ValuesAndCounts()
}

func (regularDist *RegularDistribution2) Unit() string {
	return regularDist.unit
}

func (regularDist *RegularDistribution2) Size() int {
	return regularDist.dp.BucketCounts().Len()
}

func (regularDist *RegularDistribution2) AddEntry(value float64, weight float64) error {
	return regularDist.AddEntryWithUnit(value, weight, regularDist.unit)
}
func (regularDist *RegularDistribution2) AddEntryWithUnit(value float64, weight float64, unit string) error {
	return fmt.Errorf("not implemented")
}
func (regularDist *RegularDistribution2) AddDistribution(distribution distribution.Distribution) {

}
func (regularDist *RegularDistribution2) AddDistributionWithWeight(distribution distribution.Distribution, weight float64) {
}
func (rd *RegularDistribution2) ConvertFromOtel(dp pmetric.HistogramDataPoint, unit string) {
}
func (rd *RegularDistribution2) ConvertToOtel(dp pmetric.HistogramDataPoint) {
}
func (rd *RegularDistribution2) Resize(int) []distribution.Distribution {
	return []distribution.Distribution{rd}
}

var _ (distribution.ClassicDistribution) = (*RegularDistribution2)(nil)

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

func NewFromOtelCWAgent(dp pmetric.HistogramDataPoint) ToCloudWatchValuesAndCounts {
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
		maximum:     dp.Max(),
		minimum:     dp.Min(),
		sampleCount: float64(dp.Count()),
		sum:         dp.Sum(),
		values:      make([]float64, 0),
		counts:      make([]float64, 0),
	}

	histogramBuckets := dp.ExplicitBounds()
	histogramCounts := dp.BucketCounts()

	// No validations - assuming valid input histogram

	naturalMapping := make(map[float64]float64)

	for i := 0; i < histogramBuckets.Len(); i++ {
		bucket := histogramBuckets.At(i)
		sampleCount := histogramCounts.At(i)

		if sampleCount == 0 {
			continue
		}

		// Determine bucket bounds
		prevBucket := em.minimum // If we are the first bucket, we take min...
		if i > 0 && histogramBuckets.At(i-1) > em.minimum {
			prevBucket = histogramBuckets.At(i - 1)
		}

		nextBucket := em.maximum // If we are the last bucket, take max...
		if i < histogramBuckets.Len()-1 {
			nextBucket = histogramBuckets.At(i + 1)
		}

		magnitude := -1.0 // If we are at the last bucket, we use e^-x
		if i < histogramBuckets.Len()-1 {
			nextSampleCount := histogramCounts.At(i + 1)
			magnitude = math.Log(((bucket - prevBucket) / float64(sampleCount)) / ((nextBucket - bucket) / float64(nextSampleCount)))
		}

		innerBucketCount := int(min(sampleCount, 50))
		delta := (bucket - prevBucket) / float64(innerBucketCount)
		innerHistogram := make(map[float64]float64)

		if magnitude < 0 { // Use -yx^2
			sigma := 0.0
			for j := 1; j <= innerBucketCount; j++ {
				sigma += math.Pow(float64(j), 2.0)
			}

			epsilon := float64(sampleCount) / sigma

			for j := 0; j < innerBucketCount; j++ {
				innerBucketSampleCount := epsilon * math.Pow(float64(j-innerBucketCount), 2.0)
				if j%2 == 0 {
					innerBucketSampleCount = math.Ceil(innerBucketSampleCount)
				} else {
					innerBucketSampleCount = math.Floor(innerBucketSampleCount)
				}

				if innerBucketSampleCount > 0 {
					innerHistogram[prevBucket+delta*float64(j+1)] = innerBucketSampleCount
				}
			}
		} else if magnitude < 1 { // Use x
			for j := 1; j <= innerBucketCount; j++ {
				innerHistogram[prevBucket+delta*float64(j)] = float64(sampleCount) / float64(innerBucketCount)
			}
		} else { // Use yx^2
			sigma := 0.0
			for j := 1; j <= innerBucketCount; j++ {
				sigma += math.Pow(float64(j), 2.0)
			}
			epsilon := float64(sampleCount) / sigma

			for j := 0; j < innerBucketCount; j++ {
				innerBucketSampleCount := epsilon * math.Pow(float64(j), 2.0)
				if j%2 == 0 {
					innerBucketSampleCount = math.Ceil(innerBucketSampleCount)
				} else {
					innerBucketSampleCount = math.Floor(innerBucketSampleCount)
				}

				if innerBucketSampleCount > 0 {
					innerHistogram[prevBucket+delta*float64(j+1)] = innerBucketSampleCount
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

	// extra processing not part of original implementation to convert to values/counts
	keys := slices.Collect(maps.Keys(naturalMapping))
	slices.Sort(keys)
	em.values = make([]float64, len(keys))
	em.counts = make([]float64, len(keys))
	for i, k := range keys {
		em.values[i] = k
		em.counts[i] = naturalMapping[k]
	}
	// extra processing not part of original implementation to convert to values/counts

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

// NewExponentialMappingCWFromOtel converts an OpenTelemetry histogram data point to CloudWatch values and counts format.
// It distributes bucket samples across inner buckets using density-based algorithms for optimal representation.
func NewExponentialMappingCWFromOtel(dp pmetric.HistogramDataPoint) ToCloudWatchValuesAndCounts {

	const maximumInnerBucketCount = 10

	// No validations - assuming valid input histogram

	em := &ExponentialMappingCW{
		maximum:     dp.Max(),
		minimum:     dp.Min(),
		sampleCount: float64(dp.Count()),
		sum:         dp.Sum(),
	}

	// bounds specifies the boundaries between buckets
	// bucketCounts specifies the number of datapoints in each bucket
	// there is always 1 more bucket count than there is boundaries
	// len(bucketCounts) = len(bounds) + 1
	bounds := dp.ExplicitBounds()
	lenBounds := bounds.Len()
	bucketCounts := dp.BucketCounts()
	lenBucketCounts := bucketCounts.Len()

	if !dp.HasMin() {
		// No minimum implies open lower boundary on first bucket
		// Cap lower bound to top end of first bucket.
		if lenBounds > 0 {
			em.minimum = bounds.At(0)
		} else {
			em.minimum = math.Inf(-1)
		}
	}

	if !dp.HasMax() {
		if lenBounds > 0 {
			em.maximum = bounds.At(lenBounds - 1)
		} else {
			em.minimum = math.Inf(+1)
		}
	}

	// Special case: no boundaries implies a single bucket
	if lenBounds == 0 {
		em.counts = append(em.counts, float64(bucketCounts.At(0))) // recall that len(bucketCounts) = len(bounds)+1
		if dp.HasMax() && dp.HasMin() {
			em.values = append(em.values, em.minimum/2.0+em.maximum/2.0) // overflow safe average calculation
		} else if dp.HasMax() {
			em.values = append(em.values, em.maximum) // only data point we have is the maximum
		} else if dp.HasMin() {
			em.values = append(em.values, em.minimum) // only data point we have is the minimum
		} else {
			em.values = append(em.values, 0) // arbitrary value
		}
		return em
	}

	// Pre-calculate total output size to avoid dynamic growth
	totalOutputSize := 0
	for i := 0; i < lenBucketCounts; i++ {
		sampleCount := bucketCounts.At(i)
		if sampleCount > 0 {
			totalOutputSize += int(min(sampleCount, maximumInnerBucketCount))
		}
	}
	if totalOutputSize == 0 {
		// No samples in any bucket
		return em
	}

	em.values = make([]float64, 0, totalOutputSize)
	em.counts = make([]float64, 0, totalOutputSize)

	for i := 0; i < lenBucketCounts; i++ {
		sampleCount := int(bucketCounts.At(i))
		if sampleCount == 0 {
			// No need to operate on a bucket with no samples
			continue
		}

		lowerBound := em.minimum
		if i > 0 {
			lowerBound = bounds.At(i - 1)
		}
		upperBound := em.maximum
		if i < lenBucketCounts-1 {
			upperBound = bounds.At(i)
		}
		if upperBound == lowerBound {
			if lenBounds > 1 {
				// Use width of closest defined bucket
				if i == 0 && lenBounds > 1 {
					// First bucket: use width of second bucket
					bucketWidth := bounds.At(1) - bounds.At(0)
					lowerBound = upperBound - bucketWidth
				} else if i == lenBucketCounts-1 && lenBounds > 1 {
					// Last bucket: use width of penultimate bucket
					bucketWidth := bounds.At(lenBounds-1) - bounds.At(lenBounds-2)
					upperBound = lowerBound + bucketWidth
				}
			} else {
				// Fallback: create minimal width
				if i == 0 {
					lowerBound = upperBound - 0.001
				} else {
					upperBound = lowerBound + 0.001
				}
			}
		}

		// This algorithm creates "inner buckets" between user-defined bucket based on the sample count, up to a
		// maximum. A logarithmic ratio (named "magnitude") compares the density between the current bucket and the
		// next bucket. This logarithmic ratio is used to decide how to spread samples amongst inner buckets.
		//
		// case 1: magnitude < 0
		//   * What this means: Current bucket is denser than the next bucket -> density is decreasing.
		//   * What we do: Use inverse quadratic distribution to spread the samples. This allocates more samples towards
		//     the lower bound of the bucket.
		// case 2: 0 <= magnitude < 1
		//   * What this means: Current bucket and next bucket has similar densities -> density is not changing much.
		//   * What we do: Use inform distribution to spread the samples. Extra samples that can't be spread evenly are
		//     (arbitrarily) allocated towards the start of the bucket.
		// case 3: 1 <= magnitude
		//   * What this means: Current bucket is less dense than the next bucket -> density is increasing.
		//   * What we do: Use quadratic distribution to spread the samples. This allocates more samples toward the end
		//     of the bucket.
		//
		// As a small optimization, we omit the logarithm invocation and change the thresholds.
		ratio := 0.0
		if i < lenBucketCounts-1 {
			nextSampleCount := bucketCounts.At(i + 1)
			// If next bucket is empty, than density is surely decreasing
			if nextSampleCount == 0 {
				ratio = 0.0
			} else {
				var nextUpperBound float64
				if i+1 == lenBucketCounts-1 {
					nextUpperBound = em.maximum
				} else {
					nextUpperBound = bounds.At(i + 1)
				}

				//currentBucketDensity := float64(sampleCount) / (upperBound - lowerBound)
				//nextBucketDensity := float64(nextSampleCount) / (nextUpperBound - upperBound)
				//ratio = nextBucketDensity / currentBucketDensity

				// the following calculations are the same but improves speed by ~1% in benchmark tests
				denom := (nextUpperBound - upperBound) * float64(sampleCount)
				numerator := (upperBound - lowerBound) * float64(nextSampleCount)
				ratio = numerator / denom
			}
		}

		// innerBucketCount is how many "inner buckets" to spread the sample count amongst
		innerBucketCount := min(sampleCount, maximumInnerBucketCount)
		delta := (upperBound - lowerBound) / float64(innerBucketCount)

		if ratio < 1.0/math.E { // magnitude < 0: Use -yx^2 (inverse quadratic)
			sigma := float64(sumOfSquares(innerBucketCount))
			epsilon := float64(sampleCount) / sigma
			entryStart := len(em.counts)

			runningSum := 0
			for j := 0; j < innerBucketCount; j++ {
				innerBucketSampleCount := epsilon * float64((j-innerBucketCount)*(j-innerBucketCount))
				innerBucketSampleCountAdjusted := int(math.Floor(innerBucketSampleCount))
				runningSum += innerBucketSampleCountAdjusted
				em.values = append(em.values, lowerBound+delta*float64(j+1))
				em.counts = append(em.counts, float64(innerBucketSampleCountAdjusted))
			}

			// distribute the remainder towards the front
			remainder := sampleCount - runningSum
			for j := 0; j < remainder; j++ {
				em.counts[entryStart] += 1
				entryStart += 1
			}

		} else if ratio < math.E { // 0 <= magnitude < 1: Use x
			// Distribute samples evenly with integer counts
			baseCount := sampleCount / innerBucketCount
			remainder := sampleCount % innerBucketCount
			for j := 1; j <= innerBucketCount; j++ {
				count := baseCount

				// Distribute remainder to first few buckets
				if j <= remainder {
					count++
				}
				em.values = append(em.values, lowerBound+delta*float64(j))
				em.counts = append(em.counts, float64(count))
			}

		} else { // magnitude >= 1: Use yx^2 (quadratic)
			sigma := float64(sumOfSquares(innerBucketCount))
			epsilon := float64(sampleCount) / sigma

			runningSum := 0
			for j := 1; j <= innerBucketCount; j++ {
				innerBucketSampleCount := epsilon * float64(j*j)
				innerBucketSampleCountAdjusted := int(math.Floor(innerBucketSampleCount))
				runningSum += innerBucketSampleCountAdjusted
				em.values = append(em.values, lowerBound+delta*float64(j))
				em.counts = append(em.counts, float64(innerBucketSampleCountAdjusted))
			}

			// distribute the remainder towards the end
			entryStart := len(em.counts) - 1
			remainder := sampleCount - runningSum
			for j := 0; j < remainder; j++ {
				em.counts[entryStart] += 1
				entryStart -= 1
			}
		}

	}

	// Move last entry to maximum if needed
	if dp.HasMax() && len(em.values) > 0 {
		lastIdx := len(em.values) - 1
		for i := len(em.counts) - 1; i >= 0; i-- {
			if em.counts[i] > 0 {
				lastIdx = i
				break
			}
		}
		em.values[lastIdx] = em.maximum
		em.values = em.values[:lastIdx+1]
		em.counts = em.counts[:lastIdx+1]
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

// sumOfSquares is a closed form calculation of Σx^2, for 1 to n
func sumOfSquares(n int) int {
	return n * (n + 1) * (2*n + 1) / 6
}
