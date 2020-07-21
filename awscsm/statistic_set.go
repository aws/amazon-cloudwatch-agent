// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsmmetrics

import (
	"fmt"
	"math"
)

type StatisticSet struct {
	SampleCount float64
	Sum         float64
	Min         float64
	Max         float64
}

func NewStatisticSet(value float64) StatisticSet {
	return StatisticSet{
		SampleCount: 1.0,
		Sum:         value,
		Min:         value,
		Max:         value,
	}
}

func NewWeightedStatisticSet(value float64, weight float64) StatisticSet {
	return StatisticSet{
		SampleCount: weight,
		Sum:         value * weight,
		Min:         value,
		Max:         value,
	}
}

var errNegativeSampleCount = fmt.Errorf("Statistic set cannot have a negative sample count")

// Merges two statistic set distributions
//
// Based on the following assumptions about IEEE fp math/representations:
//   (1) All finite floating point numbers are exactly one of == 0, < 0, or > 0
//   (2) Adding a (representable) positive number to a non-negative number results in a positive number
//
// we can use a SampleCount of 0 as a marker for an empty distribution since
// we only allow the merging of non-negative sample counts
func (this *StatisticSet) Merge(other StatisticSet) error {

	// invalid distributions generate an error
	if other.SampleCount < 0 || this.SampleCount < 0 {
		return errNegativeSampleCount
	}

	// empty other causes no side-effect
	if other.SampleCount == 0 {
		return nil
	}

	if this.SampleCount == 0 {
		// empty this become a copy of (non-empty) other
		this.Max = other.Max
		this.Min = other.Min
		this.SampleCount = other.SampleCount
		this.Sum = other.Sum
	} else {
		// two non-empty distributions combine in the canonical manner
		this.Max = math.Max(this.Max, other.Max)
		this.Min = math.Min(this.Min, other.Min)
		this.SampleCount += other.SampleCount
		this.Sum += other.Sum
	}

	return nil
}
