// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"reflect"
	"testing"

	awscsmmetrics "github.com/aws/amazon-cloudwatch-agent/awscsm"
	"github.com/aws/amazon-cloudwatch-agent/awscsm/sdkmetricsdataplane"
	"github.com/aws/aws-sdk-go/aws"
)

func TestAdaptFrequencyMetrics(t *testing.T) {
	cases := []struct {
		metrics  []awscsmmetrics.FrequencyMetric
		expected []*sdkmetricsdataplane.FrequencyMetric
	}{
		{
			metrics: []awscsmmetrics.FrequencyMetric{
				{
					Name: "foo",
					Frequencies: map[string]int64{
						"0":    0,
						"1":    1,
						"10":   10,
						"1000": 1000,
					},
				},
			},
			expected: []*sdkmetricsdataplane.FrequencyMetric{
				{
					Name: aws.String("foo"),
					Distribution: []*sdkmetricsdataplane.FrequencyDistributionEntry{
						{
							Key:   aws.String("0"),
							Count: aws.Int64(0),
						},
						{
							Key:   aws.String("1"),
							Count: aws.Int64(1),
						},
						{
							Key:   aws.String("10"),
							Count: aws.Int64(10),
						},
						{
							Key:   aws.String("1000"),
							Count: aws.Int64(1000),
						},
					},
				},
			},
		},
		{
			metrics: []awscsmmetrics.FrequencyMetric{
				{
					Name: "foo",
					Frequencies: map[string]int64{
						"0":    0,
						"1":    1,
						"10":   10,
						"1000": 1000,
					},
				},
				{
					Name: "bar",
					Frequencies: map[string]int64{
						"642": 1021,
						"3":   6,
						"9":   4,
						"12":  10,
						"10":  10,
						"11":  10,
					},
				},
			},
			expected: []*sdkmetricsdataplane.FrequencyMetric{
				{
					Name: aws.String("foo"),
					Distribution: []*sdkmetricsdataplane.FrequencyDistributionEntry{
						{
							Key:   aws.String("0"),
							Count: aws.Int64(0),
						},
						{
							Key:   aws.String("1"),
							Count: aws.Int64(1),
						},
						{
							Key:   aws.String("10"),
							Count: aws.Int64(10),
						},
						{
							Key:   aws.String("1000"),
							Count: aws.Int64(1000),
						},
					},
				},
				{
					Name: aws.String("bar"),
					Distribution: []*sdkmetricsdataplane.FrequencyDistributionEntry{
						{
							Key:   aws.String("10"),
							Count: aws.Int64(10),
						},
						{
							Key:   aws.String("11"),
							Count: aws.Int64(10),
						},
						{
							Key:   aws.String("12"),
							Count: aws.Int64(10),
						},
						{
							Key:   aws.String("3"),
							Count: aws.Int64(6),
						},
						{
							Key:   aws.String("642"),
							Count: aws.Int64(1021),
						},
						{
							Key:   aws.String("9"),
							Count: aws.Int64(4),
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		metrics := adaptToCSMFrequencyMetrics(c.metrics)

		if e, a := len(c.expected), len(metrics); e != a {
			t.Errorf("expected %d array size, but received %d", e, a)
		}

		for i := 0; i < len(metrics); i++ {
			m1 := metrics[i]
			m2 := c.expected[i]

			m1Dist := map[string]int64{}
			m2Dist := map[string]int64{}

			for _, v := range m1.Distribution {
				m1Dist[*v.Key] = *v.Count
			}

			for _, v := range m2.Distribution {
				m2Dist[*v.Key] = *v.Count
			}

			if !reflect.DeepEqual(m1Dist, m2Dist) {
				t.Errorf("expected %v, but received %v", m2Dist, m1Dist)
			}

			if *m1.Name != *m2.Name {
				t.Errorf("expected %v, but received %v", *m2.Name, *m1.Name)
			}

		}
	}
}

func TestAdaptSEHMetrics(t *testing.T) {
	cases := []struct {
		metrics  []awscsmmetrics.SEHMetric
		expected []*sdkmetricsdataplane.SehMetric
	}{
		{
			metrics: []awscsmmetrics.SEHMetric{
				{
					Name: "foo",
					Stats: awscsmmetrics.StatisticSet{
						SampleCount: 0.0,
						Sum:         1.0,
						Min:         2.0,
						Max:         3.0,
					},
					Buckets: map[int64]float64{
						0:  0.0,
						1:  10.0,
						2:  20.0,
						10: 100.0,
					},
				},
			},
			expected: []*sdkmetricsdataplane.SehMetric{
				{
					Name: aws.String("foo"),
					SehBuckets: []*sdkmetricsdataplane.SehBucket{
						{
							Index: aws.Int64(0),
							Value: aws.Float64(0.0),
						},
						{
							Index: aws.Int64(1),
							Value: aws.Float64(10.0),
						},
						{
							Index: aws.Int64(2),
							Value: aws.Float64(20.0),
						},
						{
							Index: aws.Int64(10),
							Value: aws.Float64(100.0),
						},
					},
					Stats: &sdkmetricsdataplane.StatisticSet{
						Count: aws.Float64(0.0),
						Sum:   aws.Float64(1.0),
						Min:   aws.Float64(2.0),
						Max:   aws.Float64(3.0),
					},
				},
			},
		},
		{
			metrics: []awscsmmetrics.SEHMetric{
				{
					Name: "foo",
					Stats: awscsmmetrics.StatisticSet{
						SampleCount: 0.0,
						Sum:         1.0,
						Min:         2.0,
						Max:         3.0,
					},
					Buckets: map[int64]float64{
						0:  0.0,
						1:  10.1,
						2:  20.2,
						10: 100.01,
					},
				},
				{
					Name: "bar",
					Stats: awscsmmetrics.StatisticSet{
						SampleCount: 15.0,
						Sum:         255.0,
						Min:         1.0,
						Max:         300.0,
					},
					Buckets: map[int64]float64{
						0:    0.0,
						1024: 10.0,
						2:    2048.0,
						10:   100.0,
					},
				},
			},
			expected: []*sdkmetricsdataplane.SehMetric{
				{
					Name: aws.String("foo"),
					SehBuckets: []*sdkmetricsdataplane.SehBucket{
						{
							Index: aws.Int64(0),
							Value: aws.Float64(0.0),
						},
						{
							Index: aws.Int64(1),
							Value: aws.Float64(10.1),
						},
						{
							Index: aws.Int64(2),
							Value: aws.Float64(20.2),
						},
						{
							Index: aws.Int64(10),
							Value: aws.Float64(100.01),
						},
					},
					Stats: &sdkmetricsdataplane.StatisticSet{
						Count: aws.Float64(0.0),
						Sum:   aws.Float64(1.0),
						Min:   aws.Float64(2.0),
						Max:   aws.Float64(3.0),
					},
				},
				{
					Name: aws.String("bar"),
					SehBuckets: []*sdkmetricsdataplane.SehBucket{
						{
							Index: aws.Int64(0),
							Value: aws.Float64(0.0),
						},
						{
							Index: aws.Int64(2),
							Value: aws.Float64(2048.0),
						},
						{
							Index: aws.Int64(10),
							Value: aws.Float64(100.0),
						},
						{
							Index: aws.Int64(1024),
							Value: aws.Float64(10.0),
						},
					},
					Stats: &sdkmetricsdataplane.StatisticSet{
						Count: aws.Float64(15.0),
						Sum:   aws.Float64(255.0),
						Min:   aws.Float64(1.0),
						Max:   aws.Float64(300.0),
					},
				},
			},
		},
	}

	for _, c := range cases {
		metrics := adaptToCSMSEHMetrics(c.metrics)
		if e, a := len(c.expected), len(metrics); e != a {
			t.Errorf("expected %d array size, but received %d", e, a)
		}

		outputAsMap := make(map[string]*sdkmetricsdataplane.SehMetric)
		for i := 0; i < len(metrics); i++ {
			sdkMetric := metrics[i]
			outputAsMap[*sdkMetric.Name] = sdkMetric
		}

		if len(outputAsMap) != len(c.expected) {
			t.Errorf("expected output size %v, but received %v", len(c.expected), len(outputAsMap))
		}

		for i := 0; i < len(c.expected); i++ {
			m2 := c.expected[i]
			m1 := outputAsMap[*m2.Name]

			m1Buckets := map[int64]float64{}
			m2Buckets := map[int64]float64{}

			for _, v := range m1.SehBuckets {
				m1Buckets[*v.Index] = *v.Value
			}

			for _, v := range m2.SehBuckets {
				m2Buckets[*v.Index] = *v.Value
			}

			if !reflect.DeepEqual(m1Buckets, m2Buckets) {
				t.Errorf("expected %v, but received %v", m2Buckets, m1Buckets)
			}

			if !reflect.DeepEqual(m1.Stats, m2.Stats) {
				t.Errorf("expected %v, but received %v", m2.Stats, m1.Stats)
			}
		}
	}
}
