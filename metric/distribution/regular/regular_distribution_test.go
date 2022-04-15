// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package regular

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSEH1Distribution(t *testing.T) {
	//dist new and add entry
	dist := NewRegularDistribution()

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
	valuesCountsMap := map[float64]float64{}
	for i := 0; i < len(values); i++ {
		valuesCountsMap[values[i]] = counts[i]
	}
	expectedValuesCountsMap := map[float64]float64{20: 1, 30: 1, 50: 1}
	assert.Equal(t, expectedValuesCountsMap, valuesCountsMap)

	//another dist new and add entry
	anotherDist := NewRegularDistribution()

	anotherDist.AddEntry(21, 1)
	anotherDist.AddEntry(22, 1)
	anotherDist.AddEntry(23, 2)

	assert.Equal(t, 89.0, anotherDist.Sum())
	assert.Equal(t, 4.0, anotherDist.SampleCount())
	assert.Equal(t, 21.0, anotherDist.Minimum())
	assert.Equal(t, 23.0, anotherDist.Maximum())
	assert.Equal(t, "", anotherDist.Unit())
	values, counts = anotherDist.ValuesAndCounts()
	assert.Equal(t, len(values), len(counts))
	valuesCountsMap = map[float64]float64{}
	for i := 0; i < len(values); i++ {
		valuesCountsMap[values[i]] = counts[i]
	}
	expectedValuesCountsMap = map[float64]float64{21: 1, 22: 1, 23: 2}
	assert.Equal(t, expectedValuesCountsMap, valuesCountsMap)

	//clone dist and anotherDist
	distClone := cloneRegularDistribution(dist.(*RegularDistribution))

	//add another dist into dist
	dist.AddDistribution(anotherDist)

	assert.Equal(t, 189.0, dist.Sum())
	assert.Equal(t, 7.0, dist.SampleCount())
	assert.Equal(t, 20.0, dist.Minimum())
	assert.Equal(t, 50.0, dist.Maximum())
	assert.Equal(t, "Count", dist.Unit())
	values, counts = dist.ValuesAndCounts()
	assert.Equal(t, len(values), len(counts))
	valuesCountsMap = map[float64]float64{}
	for i := 0; i < len(values); i++ {
		valuesCountsMap[values[i]] = counts[i]
	}
	expectedValuesCountsMap = map[float64]float64{20: 1, 21: 1, 22: 1, 23: 2, 30: 1, 50: 1}
	assert.Equal(t, expectedValuesCountsMap, valuesCountsMap)

	//add distClone into another dist
	anotherDist.AddDistribution(distClone)
	assert.Equal(t, dist, anotherDist) //the direction of AddDistribution should not matter.
}

func cloneRegularDistribution(dist *RegularDistribution) *RegularDistribution {
	clonedDist := &RegularDistribution{
		maximum:     dist.maximum,
		minimum:     dist.minimum,
		sampleCount: dist.sampleCount,
		sum:         dist.sum,
		buckets:     map[float64]float64{},
		unit:        dist.unit,
	}
	for k, v := range dist.buckets {
		clonedDist.buckets[k] = v
	}
	return clonedDist
}
