// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package regular

import (
	"fmt"
	"log"
	"math"
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

func NewRegularDistribution() distribution.Distribution {
	return &RegularDistribution{
		maximum:     0, // negative number is not supported for now, so zero is the min value
		minimum:     math.MaxFloat64,
		sampleCount: 0,
		sum:         0,
		buckets:     map[float64]float64{},
		unit:        "",
	}
}

func (rd *RegularDistribution) Maximum() float64 {
	return rd.maximum
}

func (rd *RegularDistribution) Minimum() float64 {
	return rd.minimum
}

func (rd *RegularDistribution) SampleCount() float64 {
	return rd.sampleCount
}

func (rd *RegularDistribution) Sum() float64 {
	return rd.sum
}

func (rd *RegularDistribution) ValuesAndCounts() (values []float64, counts []float64) {
	values = []float64{}
	counts = []float64{}
	for value, counter := range rd.buckets {
		values = append(values, value)
		counts = append(counts, counter)
	}
	return
}

func (rd *RegularDistribution) Unit() string {
	return rd.unit
}

func (rd *RegularDistribution) Size() int {
	return len(rd.buckets)
}

// weight is 1/samplingRate
func (rd *RegularDistribution) AddEntryWithUnit(value float64, weight float64, unit string) error {
	if weight <= 0 {
		return fmt.Errorf("unsupported weight %v: %w", weight, distribution.ErrUnsupportedWeight)
	}
	if !distribution.IsSupportedValue(value, 0, distribution.MaxValue) {
		return fmt.Errorf("unsupported value %v: %w", value, distribution.ErrUnsupportedValue)
	}
	//sample count
	rd.sampleCount += weight
	//sum
	rd.sum += value * weight
	//min
	if value < rd.minimum {
		rd.minimum = value
	}
	//max
	if value > rd.maximum {
		rd.maximum = value
	}

	//values and counts
	rd.buckets[value] += weight

	//unit
	if rd.unit == "" {
		rd.unit = unit
	} else if rd.unit != unit && unit != "" {
		log.Printf("D! Multiple units are detected: %s, %s", rd.unit, unit)
	}
	return nil
}

// weight is 1/samplingRate
func (rd *RegularDistribution) AddEntry(value float64, weight float64) error {
	return rd.AddEntryWithUnit(value, weight, "")
}

func (rd *RegularDistribution) AddDistribution(distribution distribution.Distribution) {
	rd.AddDistributionWithWeight(distribution, 1)
}

func (rd *RegularDistribution) AddDistributionWithWeight(distribution distribution.Distribution, weight float64) {
	if distribution.SampleCount()*weight > 0 {

		//values and counts
		if fromDistribution, ok := distribution.(*RegularDistribution); ok {
			for bucketNumber, bucketCounts := range fromDistribution.buckets {
				rd.buckets[bucketNumber] += bucketCounts * weight
			}
		} else {
			log.Printf("E! The from distribution type is not compatible with the to distribution type: from distribution type %T, to distribution type %T", rd, distribution)
			return
		}

		//sample count
		rd.sampleCount += distribution.SampleCount() * weight
		//sum
		rd.sum += distribution.Sum() * weight
		//min
		if distribution.Minimum() < rd.minimum {
			rd.minimum = distribution.Minimum()
		}
		//max
		if distribution.Maximum() > rd.maximum {
			rd.maximum = distribution.Maximum()
		}

		//unit
		if rd.unit == "" {
			rd.unit = distribution.Unit()
		} else if rd.unit != distribution.Unit() && distribution.Unit() != "" {
			log.Printf("D! Multiple units are dected: %s, %s", rd.unit, distribution.Unit())
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

func (rd *RegularDistribution) Resize(listMaxSize int) []distribution.Distribution {
	distList := []distribution.Distribution{}
	values, _ := rd.ValuesAndCounts()
	sort.Float64s(values)
	newSEH1Dist := seh1.NewSEH1Distribution().(*seh1.SEH1Distribution)
	for i := 0; i < len(values); i++ {
		if !newSEH1Dist.CanAdd(values[i], listMaxSize) {
			distList = append(distList, newSEH1Dist)
			newSEH1Dist = seh1.NewSEH1Distribution().(*seh1.SEH1Distribution)
		}
		newSEH1Dist.AddEntry(values[i], rd.GetCount(values[i]))
	}
	if newSEH1Dist.Size() > 0 {
		distList = append(distList, newSEH1Dist)
	}
	return distList
}

func (rd *RegularDistribution) GetCount(value float64) float64 {
	return rd.buckets[value]
}
