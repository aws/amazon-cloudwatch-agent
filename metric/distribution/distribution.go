// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package distribution

import (
	"errors"
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

var (
	ErrUnsupportedWeight = errors.New("weight must be larger than 0")
	ErrUnsupportedValue  = errors.New("value cannot be negative, NaN, Inf, or greater than 2^360")
	MinValue             = -math.Pow(2, 360)
	MaxValue             = math.Pow(2, 360)
)

type Distribution interface {
	Maximum() float64

	Minimum() float64

	SampleCount() float64

	Sum() float64

	ValuesAndCounts() ([]float64, []float64)

	Unit() string

	Size() int

	// weight is 1/samplingRate
	AddEntryWithUnit(value float64, weight float64, unit string) error

	// weight is 1/samplingRate
	AddEntry(value float64, weight float64) error

	AddDistribution(distribution Distribution)

	AddDistributionWithWeight(distribution Distribution, weight float64)

	ConvertToOtel(dp pmetric.HistogramDataPoint)

	ConvertFromOtel(dp pmetric.HistogramDataPoint, unit string)
}

var NewDistribution func() Distribution

// IsSupportedValue checks to see if the metric is between the min value and 2^360 and not a NaN.
// This matches the accepted range described in the MetricDatum documentation
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricDatum.html
func IsSupportedValue(value, min, max float64) bool {
	return !math.IsNaN(value) && value >= min && value <= max
}
