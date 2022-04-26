// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package distribution

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
}

var NewDistribution func() Distribution
