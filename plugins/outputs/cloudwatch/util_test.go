// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatch

import (
	"fmt"
	"log"
	"sort"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/metric/distribution"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/regular"
	"github.com/aws/amazon-cloudwatch-agent/metric/distribution/seh1"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatch"
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
	_, ok := distribution.NewDistribution().(*seh1.SEH1Distribution)
	assert.True(t, ok)

	setNewDistributionFunc(defaultMaxValuesPerDatum)
	_, ok = distribution.NewDistribution().(*regular.RegularDistribution)
	assert.True(t, ok)
}

func TestResize(t *testing.T) {
	maxListSize := 2
	setNewDistributionFunc(maxListSize)

	dist := distribution.NewDistribution()

	dist.AddEntry(1, 1)

	distList := resize(dist, maxListSize)
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

	distList = resize(dist, maxListSize)
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
	datum := new(cloudwatch.MetricDatum)
	datum.SetCounts(aws.Float64Slice([]float64{1, 2, 3}))
	datum.SetValues(aws.Float64Slice([]float64{1, 2, 3}))
	datum.SetStatisticValues(&cloudwatch.StatisticSet{
		Sum:         aws.Float64(6),
		SampleCount: aws.Float64(3),
		Minimum:     aws.Float64(1),
		Maximum:     aws.Float64(3),
	})
	datum.SetDimensions([]*cloudwatch.Dimension{
		{Name: aws.String("DimensionName"), Value: aws.String("DimensionValue")},
	})
	datum.SetMetricName("MetricName")
	datum.SetStorageResolution(1)
	datum.SetTimestamp(time.Now())
	datum.SetUnit("None")
	assert.Equal(t, 867, payload(datum, nil))
}

func TestPayload_Value(t *testing.T) {
	datum := new(cloudwatch.MetricDatum)
	datum.SetValue(1.23456789)
	datum.SetDimensions([]*cloudwatch.Dimension{
		{Name: aws.String("DimensionName"), Value: aws.String("DimensionValue")},
	})
	datum.SetMetricName("MetricName")
	datum.SetStorageResolution(1)
	datum.SetTimestamp(time.Now())
	datum.SetUnit("None")
	assert.Equal(t, 356, payload(datum, nil))
}

func TestPayload_Min(t *testing.T) {
	datum := new(cloudwatch.MetricDatum)
	datum.SetValue(1.23456789)
	datum.SetMetricName("MetricName")
	datum.SetTimestamp(time.Now())
	assert.Equal(t, 148, payload(datum, nil))
}

func TestPayload_Entity(t *testing.T) {
	datum := new(cloudwatch.MetricDatum)
	datum.SetValue(1.23456789)
	datum.SetMetricName("MetricName")
	datum.SetTimestamp(time.Now())

	entity := cloudwatch.Entity{
		KeyAttributes: map[string]*string{
			"Environment": aws.String("Environment"),
			"Service":     aws.String("Service"),
		},
		Attributes: map[string]*string{
			"TestAttribute": aws.String("TestValue"),
		},
	}

	expectedDatumSize := 148
	expectedEntitySize := 133
	fmt.Println(entity.String())
	assert.Equal(t, expectedDatumSize+expectedEntitySize, payload(datum, &entity))
}

func TestEntityToString_StringToEntity(t *testing.T) {
	testCases := []struct {
		name         string
		entity       cloudwatch.Entity
		entityString string
	}{
		{
			name: "Full Entity",
			entity: cloudwatch.Entity{
				KeyAttributes: map[string]*string{
					"Service":     aws.String("Service"),
					"Environment": aws.String("Environment"),
				},
				Attributes: map[string]*string{
					"InstanceId":   aws.String("InstanceId"),
					"InstanceType": aws.String("InstanceType"),
				},
			},
			entityString: "InstanceId:InstanceId;InstanceType:InstanceType|Environment:Environment;Service:Service",
		},
		{
			name: "Empty Attributes",
			entity: cloudwatch.Entity{
				KeyAttributes: map[string]*string{
					"Service":     aws.String("Service"),
					"Environment": aws.String("Environment"),
				},
				Attributes: map[string]*string{},
			},
			entityString: "|Environment:Environment;Service:Service",
		},
		{
			name:         "Empty Entity",
			entity:       cloudwatch.Entity{},
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
		entity       cloudwatch.Entity
		entityString string
	}{
		{
			name: "EmptyEntityMaps",
			entity: cloudwatch.Entity{
				KeyAttributes: map[string]*string{},
				Attributes:    map[string]*string{},
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
