// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package seh1

import (
	"fmt"
	"log"
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
)

var bucketForZero int16 = math.MinInt16
var bucketFactor = math.Log(1 + 0.1)

type SEH1Distribution struct {
	maximum     float64
	minimum     float64
	sampleCount float64
	sum         float64
	buckets     map[int16]float64 // from bucket number (i.e. value) to the counter (i.e. weight)
	unit        string
}

func NewSEH1Distribution() distribution.Distribution {
	return &SEH1Distribution{
		maximum:     0, // negative number is not supported for now, so zero is the min value
		minimum:     math.MaxFloat64,
		sampleCount: 0,
		sum:         0,
		buckets:     map[int16]float64{},
		unit:        "",
	}
}

func (seh1Distribution *SEH1Distribution) Maximum() float64 {
	return seh1Distribution.maximum
}

func (seh1Distribution *SEH1Distribution) Minimum() float64 {
	return seh1Distribution.minimum
}

func (seh1Distribution *SEH1Distribution) SampleCount() float64 {
	return seh1Distribution.sampleCount
}

func (seh1Distribution *SEH1Distribution) Sum() float64 {
	return seh1Distribution.sum
}

func (seh1Distribution *SEH1Distribution) ValuesAndCounts() (values []float64, counts []float64) {
	values = []float64{}
	counts = []float64{}
	for bucketNumber, counter := range seh1Distribution.buckets {
		var value float64
		if bucketNumber == bucketForZero {
			value = 0
		} else {
			// Add 0.5 to calculate exponent for the middle of the bin
			value = math.Exp((float64(bucketNumber) + 0.5) * bucketFactor)
		}
		values = append(values, value)
		counts = append(counts, counter)
	}
	return
}

func (seh1Distribution *SEH1Distribution) Unit() string {
	return seh1Distribution.unit
}

func (seh1Distribution *SEH1Distribution) Size() int {
	return len(seh1Distribution.buckets)
}

// weight is 1/samplingRate
func (seh1Distribution *SEH1Distribution) AddEntryWithUnit(value float64, weight float64, unit string) error {
	if weight <= 0 {
		return fmt.Errorf("unsupported weight %v: %w", weight, distribution.ErrUnsupportedWeight)
	}
	if !distribution.IsSupportedValue(value, 0, distribution.MaxValue) {
		return fmt.Errorf("unsupported value %v: %w", value, distribution.ErrUnsupportedValue)
	}
	//sample count
	seh1Distribution.sampleCount += weight
	//sum
	seh1Distribution.sum += value * weight
	//min
	if value < seh1Distribution.minimum {
		seh1Distribution.minimum = value
	}
	//max
	if value > seh1Distribution.maximum {
		seh1Distribution.maximum = value
	}

	//seh
	bucketNumber := bucketNumber(value)
	seh1Distribution.buckets[bucketNumber] += weight

	//unit
	if seh1Distribution.unit == "" {
		seh1Distribution.unit = unit
	} else if seh1Distribution.unit != unit && unit != "" {
		log.Printf("D! Multiple units are detected: %s, %s", seh1Distribution.unit, unit)
	}
	return nil
}

// weight is 1/samplingRate
func (seh1Distribution *SEH1Distribution) AddEntry(value float64, weight float64) error {
	return seh1Distribution.AddEntryWithUnit(value, weight, "")
}

func (seh1Distribution *SEH1Distribution) AddDistribution(distribution distribution.Distribution) {
	seh1Distribution.AddDistributionWithWeight(distribution, 1)
}

func (seh1Distribution *SEH1Distribution) AddDistributionWithWeight(distribution distribution.Distribution, weight float64) {
	if distribution.SampleCount()*weight > 0 {

		//seh
		if fromSEH1Distribution, ok := distribution.(*SEH1Distribution); ok {
			for bucketNumber, bucketCounts := range fromSEH1Distribution.buckets {
				seh1Distribution.buckets[bucketNumber] += bucketCounts * weight
			}
		} else {
			log.Printf("E! The from distribution type is not compatible with the to distribution type: from distribution type %T, to distribution type %T", seh1Distribution, distribution)
			return
		}

		//sample count
		seh1Distribution.sampleCount += distribution.SampleCount() * weight
		//sum
		seh1Distribution.sum += distribution.Sum() * weight
		//min
		if distribution.Minimum() < seh1Distribution.minimum {
			seh1Distribution.minimum = distribution.Minimum()
		}
		//max
		if distribution.Maximum() > seh1Distribution.maximum {
			seh1Distribution.maximum = distribution.Maximum()
		}

		//unit
		if seh1Distribution.unit == "" {
			seh1Distribution.unit = distribution.Unit()
		} else if seh1Distribution.unit != distribution.Unit() && distribution.Unit() != "" {
			log.Printf("D! Multiple units are detected: %s, %s", seh1Distribution.unit, distribution.Unit())
		}
	} else {
		log.Printf("D! SampleCount * Weight should be larger than 0: %v, %v", distribution.SampleCount(), weight)
	}
}

// ConvertToOtel could convert an SEH1Distribution to pmetric.ExponentialHistogram.
// But there is no need because it will just get converted bak to a SEH1Distribution.
func (sd *SEH1Distribution) ConvertToOtel(dp pmetric.HistogramDataPoint) {
	dp.SetMax(sd.maximum)
	dp.SetMin(sd.minimum)
	dp.SetCount(uint64(sd.sampleCount))
	dp.SetSum(sd.sum)
	dp.ExplicitBounds().EnsureCapacity(len(sd.buckets))
	dp.BucketCounts().EnsureCapacity(len(sd.buckets))
	for k, v := range sd.buckets {
		dp.ExplicitBounds().Append(float64(k))
		// Beware of potential loss of precision due to type conversion.
		dp.BucketCounts().Append(uint64(v))
	}
}

func (sd *SEH1Distribution) ConvertFromOtel(dp pmetric.HistogramDataPoint, unit string) {
	sd.maximum = dp.Max()
	sd.minimum = dp.Min()
	sd.sampleCount = float64(dp.Count())
	sd.sum = dp.Sum()
	sd.unit = unit
	for i := 0; i < dp.ExplicitBounds().Len(); i++ {
		k := dp.ExplicitBounds().At(i)
		v := dp.BucketCounts().At(i)
		sd.buckets[int16(k)] = float64(v)
	}
}

func (seh1Distribution *SEH1Distribution) CanAdd(value float64, sizeLimit int) bool {
	if seh1Distribution.Size() < sizeLimit {
		return true
	}
	bucketNumber := bucketNumber(value)
	if _, ok := seh1Distribution.buckets[bucketNumber]; ok {
		return true
	}
	return false
}

func bucketNumber(value float64) int16 {
	bucketNumber := bucketForZero
	if value > 0 {
		bucketNumber = int16(floor(math.Log(value) / bucketFactor))
	}
	return bucketNumber
}

// This method is faster than math.Floor
func floor(fvalue float64) int64 {
	ivalue := int64(fvalue)
	if fvalue < 0 && float64(ivalue) != fvalue {
		ivalue--
	}
	return ivalue
}
