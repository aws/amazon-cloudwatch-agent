// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package seh1

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
)

func TestSEH1Distribution(t *testing.T) {
	//dist new and add entry
	dist := NewSEH1Distribution()

	assert.NoError(t, dist.AddEntry(20, 1))
	assert.NoError(t, dist.AddEntry(30, 1))
	assert.NoError(t, dist.AddEntryWithUnit(50, 1, "Count"))

	assert.Equal(t, 100.0, dist.Sum())
	assert.Equal(t, 3.0, dist.SampleCount())
	assert.Equal(t, 20.0, dist.Minimum())
	assert.Equal(t, 50.0, dist.Maximum())
	assert.Equal(t, "Count", dist.Unit())
	values, counts := dist.ValuesAndCounts()
	assert.Equal(t, len(values), len(counts))
	valuesCountsMap := map[string]float64{}
	for i := 0; i < len(values); i++ {
		valuesCountsMap[truncate(values[i])] = counts[i]
	}
	expectedValuesCountsMap := map[string]float64{"20.13119624": 1, "29.47408442": 1, "52.21513847": 1}

	assert.Equal(t, expectedValuesCountsMap, valuesCountsMap)

	//another dist new and add entry
	anotherDist := NewSEH1Distribution()

	assert.NoError(t, anotherDist.AddEntry(21, 1))
	assert.NoError(t, anotherDist.AddEntry(22, 1))
	assert.NoError(t, anotherDist.AddEntry(23, 2))

	assert.Equal(t, 89.0, anotherDist.Sum())
	assert.Equal(t, 4.0, anotherDist.SampleCount())
	assert.Equal(t, 21.0, anotherDist.Minimum())
	assert.Equal(t, 23.0, anotherDist.Maximum())
	assert.Equal(t, "", anotherDist.Unit())
	values, counts = anotherDist.ValuesAndCounts()
	assert.Equal(t, len(values), len(counts))
	valuesCountsMap = map[string]float64{}
	for i := 0; i < len(values); i++ {
		valuesCountsMap[truncate(values[i])] = counts[i]
	}
	expectedValuesCountsMap = map[string]float64{"20.13119624": 1, "22.14431587": 3}
	assert.Equal(t, expectedValuesCountsMap, valuesCountsMap)

	//clone dist and anotherDist
	distClone := cloneSEH1Distribution(dist.(*SEH1Distribution))

	//add another dist into dist
	dist.AddDistribution(anotherDist)

	assert.Equal(t, 189.0, dist.Sum())
	assert.Equal(t, 7.0, dist.SampleCount())
	assert.Equal(t, 20.0, dist.Minimum())
	assert.Equal(t, 50.0, dist.Maximum())
	assert.Equal(t, "Count", dist.Unit())
	values, counts = dist.ValuesAndCounts()
	assert.Equal(t, len(values), len(counts))
	valuesCountsMap = map[string]float64{}
	for i := 0; i < len(values); i++ {
		valuesCountsMap[truncate(values[i])] = counts[i]
	}
	expectedValuesCountsMap = map[string]float64{"20.13119624": 2, "22.14431587": 3, "29.47408442": 1, "52.21513847": 1}
	assert.Equal(t, expectedValuesCountsMap, valuesCountsMap)

	//add distClone into another dist
	anotherDist.AddDistribution(distClone)
	assert.Equal(t, dist, anotherDist) //the direction of AddDistribution should not matter.

	assert.ErrorIs(t, anotherDist.AddEntry(1, 0), distribution.ErrUnsupportedWeight)
	assert.ErrorIs(t, anotherDist.AddEntry(-1, 1), distribution.ErrUnsupportedValue)
	assert.ErrorIs(t, anotherDist.AddEntry(math.NaN(), 1), distribution.ErrUnsupportedValue)
	assert.ErrorIs(t, anotherDist.AddEntry(math.Inf(1), 1), distribution.ErrUnsupportedValue)
	assert.ErrorIs(t, anotherDist.AddEntry(math.Inf(-1), 1), distribution.ErrUnsupportedValue)
	assert.ErrorIs(t, anotherDist.AddEntry(distribution.MaxValue*1.001, 1), distribution.ErrUnsupportedValue)
	assert.ErrorIs(t, anotherDist.AddEntry(distribution.MinValue*1.001, 1), distribution.ErrUnsupportedValue)
}

func cloneSEH1Distribution(dist *SEH1Distribution) *SEH1Distribution {
	clonedDist := &SEH1Distribution{
		maximum:     dist.maximum,
		minimum:     dist.minimum,
		sampleCount: dist.sampleCount,
		sum:         dist.sum,
		buckets:     map[int16]float64{},
		unit:        dist.unit,
	}
	for k, v := range dist.buckets {
		clonedDist.buckets[k] = v
	}
	return clonedDist
}

func truncate(f float64) string {
	return big.NewFloat(f).SetPrec(100).String()
}
