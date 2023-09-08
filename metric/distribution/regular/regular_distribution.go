// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package regular

import (
	"fmt"
	"log"
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
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
	for value, counter := range regularDist.buckets {
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

func (regularDist *RegularDistribution) GetCount(value float64) float64 {
	return regularDist.buckets[value]
}
