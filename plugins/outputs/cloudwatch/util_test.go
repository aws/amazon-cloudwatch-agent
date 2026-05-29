// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"log"
	"sort"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/regular"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/seh1"
)

func TestPublishJitter(t *testing.T) {
	// Loop an arbitrary number of times.
	last := time.Duration(-1)
	for i := 0; i < 100; i++ {
		publishJitter := publishJitter(time.Minute)
		log.Printf("Got publisherJitter %v", publishJitter)
		assert.GreaterOrEqual(t, publishJitter, time.Duration(0))
		assert.Less(t, publishJitter, time.Minute)
		assert.NotEqual(t, publishJitter, last)
		last = publishJitter
	}
}

func TestSetNewDistributionFunc(t *testing.T) {
	setNewDistributionFunc(maxValuesPerDatum)
	_, ok := distribution.NewClassicDistribution().(*seh1.SEH1Distribution)
	assert.True(t, ok)

	setNewDistributionFunc(defaultMaxValuesPerDatum)
	_, ok = distribution.NewClassicDistribution().(*regular.RegularDistribution)
	assert.True(t, ok)
}

func TestResize(t *testing.T) {
	maxListSize := 2
	setNewDistributionFunc(maxListSize)

	dist := distribution.NewClassicDistribution()

	assert.NoError(t, dist.AddEntry(1, 1))

	distList := dist.Resize(maxListSize)
	assert.Equal(t, 1, len(distList))

	actualDist := distList[0]
	values, counts := actualDist.ValuesAndCounts()
	unit := actualDist.Unit()
	maximum, minimum, sampleCount, sum := actualDist.Maximum(), actualDist.Minimum(), actualDist.SampleCount(), actualDist.Sum()

	assert.Equal(t, []float64{1.0488088481701516}, values)
	assert.Equal(t, []float64{1}, counts)
	assert.Equal(t, "", unit)
	assert.Equal(t, float64(1), maximum)
	assert.Equal(t, float64(1), minimum)
	assert.Equal(t, float64(1), sampleCount)
	assert.Equal(t, float64(1), sum)

	assert.NoError(t, dist.AddEntry(2, 1))
	assert.NoError(t, dist.AddEntry(3, 1))
	assert.NoError(t, dist.AddEntry(4, 1))

	distList = dist.Resize(maxListSize)
	assert.Equal(t, 2, len(distList))

	actualDist = distList[0]
	values, counts = actualDist.ValuesAndCounts()
	unit = actualDist.Unit()
	maximum, minimum, sampleCount, sum = actualDist.Maximum(), actualDist.Minimum(), actualDist.SampleCount(), actualDist.Sum()
	sort.Float64s(values)

	assert.Equal(t, []float64{1.0488088481701516, 2.0438317370604793}, values)
	assert.Equal(t, []float64{1, 1}, counts)
	assert.Equal(t, "", unit)
	assert.Equal(t, float64(2), maximum)
	assert.Equal(t, float64(1), minimum)
	assert.Equal(t, float64(2), sampleCount)
	assert.Equal(t, float64(3), sum)

	actualDist = distList[1]
	values, counts = actualDist.ValuesAndCounts()
	unit = actualDist.Unit()
	maximum, minimum, sampleCount, sum = actualDist.Maximum(), actualDist.Minimum(), actualDist.SampleCount(), actualDist.Sum()
	sort.Float64s(values)

	assert.Equal(t, []float64{2.992374046230249, 3.9828498555324616}, values)
	assert.Equal(t, []float64{1, 1}, counts)
	assert.Equal(t, "", unit)
	assert.Equal(t, float64(4), maximum)
	assert.Equal(t, float64(3), minimum)
	assert.Equal(t, float64(2), sampleCount)
	assert.Equal(t, float64(7), sum)
}

func TestPayload_ValuesAndCounts(t *testing.T) {
	datum := &types.MetricDatum{
		Counts: []float64{1, 2, 3},
		Values: []float64{1, 2, 3},
		StatisticValues: &types.StatisticSet{
			Sum:         aws.Float64(6),
			SampleCount: aws.Float64(3),
			Minimum:     aws.Float64(1),
			Maximum:     aws.Float64(3),
		},
		Dimensions: []types.Dimension{
			{Name: aws.String("DimensionName"), Value: aws.String("DimensionValue")},
		},
		MetricName:        aws.String("MetricName"),
		StorageResolution: aws.Int32(1),
		Timestamp:         aws.Time(time.Now()),
		Unit:              types.StandardUnitNone,
	}
	assert.Equal(t, 867, payload(datum))
}

func TestPayload_Value(t *testing.T) {
	datum := &types.MetricDatum{
		Value: aws.Float64(1.23456789),
		Dimensions: []types.Dimension{
			{Name: aws.String("DimensionName"), Value: aws.String("DimensionValue")},
		},
		MetricName:        aws.String("MetricName"),
		StorageResolution: aws.Int32(1),
		Timestamp:         aws.Time(time.Now()),
		Unit:              types.StandardUnitNone,
	}
	assert.Equal(t, 356, payload(datum))
}

func TestPayload_Min(t *testing.T) {
	datum := &types.MetricDatum{
		Value:      aws.Float64(1.23456789),
		MetricName: aws.String("MetricName"),
		Timestamp:  aws.Time(time.Now()),
	}
	assert.Equal(t, 148, payload(datum))
}

func TestEntityToString_StringToEntity(t *testing.T) {
	testCases := []struct {
		name         string
		entity       types.Entity
		entityString string
	}{
		{
			name: "Full Entity",
			entity: types.Entity{
				KeyAttributes: map[string]string{
					"Service":     "Service",
					"Environment": "Environment",
				},
				Attributes: map[string]string{
					"InstanceId":   "InstanceId",
					"InstanceType": "InstanceType",
				},
			},
			entityString: "InstanceId:InstanceId;InstanceType:InstanceType|Environment:Environment;Service:Service",
		},
		{
			name: "Empty Attributes",
			entity: types.Entity{
				KeyAttributes: map[string]string{
					"Service":     "Service",
					"Environment": "Environment",
				},
				Attributes: map[string]string{},
			},
			entityString: "|Environment:Environment;Service:Service",
		},
		{
			name:         "Empty Entity",
			entity:       types.Entity{},
			entityString: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.entityString, entityToString(tc.entity))
			assert.Equal(t, tc.entity, stringToEntity(tc.entityString))
		})
	}
}

func TestEntityToString(t *testing.T) {
	testCases := []struct {
		name         string
		entity       types.Entity
		entityString string
	}{
		{
			name: "EmptyEntityMaps",
			entity: types.Entity{
				KeyAttributes: map[string]string{},
				Attributes:    map[string]string{},
			},
			entityString: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.entityString, entityToString(tc.entity))
		})
	}
}
