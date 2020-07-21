// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsmmetrics

import (
	"reflect"
	"testing"
)

func TestZeroDistribution(t *testing.T) {
	m := NewSEHMetric("")

	if e, a := 0, len(m.Name); e != a {
		t.Errorf("expected %d but received %d", e, a)
	}

	if e, a := 0, len(m.Buckets); e != a {
		t.Errorf("expected %d but received %d", e, a)
	}

	if m.Stats.SampleCount != 0.0 {
		t.Errorf("expected 'SampleCount' to be zero, but received %f", m.Stats.SampleCount)
	}

	if m.Stats.Sum != 0.0 {
		t.Errorf("expected 'Sum' to be zero, but received %f", m.Stats.Sum)
	}

	if m.Stats.Min != 0.0 {
		t.Errorf("expected 'Min' to be zero, but received %f", m.Stats.Min)
	}

	if m.Stats.Max != 0.0 {
		t.Errorf("expected 'Max' to be zero, but received %f", m.Stats.Max)
	}
}

type Sample struct {
	Value  float64
	Weight float64
}

func TestSEHMetrics(t *testing.T) {
	cases := []struct {
		name           string
		samples        []Sample
		metric         SEHMetric
		expectedMetric SEHMetric
		expectedError  error
	}{
		{
			"zero weight",
			[]Sample{
				Sample{Value: 0.0, Weight: 0.0},
			},
			NewSEHMetric("test"),
			SEHMetric{
				Name:  "test",
				Stats: StatisticSet{SampleCount: 0.0, Sum: 0.0, Min: 0.0, Max: 0.0},
				Buckets: map[int64]float64{
					ZeroBucket: 0.0,
				},
			},
			nil,
		},
		{
			"negative weight",
			[]Sample{
				Sample{Value: 2.0, Weight: -1.0},
			},
			NewSEHMetric("test"),
			NewSEHMetric("test"),
			errNegativeSampleCount,
		},
		{
			"negative value",
			[]Sample{
				Sample{Value: -2.0, Weight: 1.0},
			},
			NewSEHMetric("test"),
			NewSEHMetric("test"),
			errNegativeSEHSampleValue,
		},
		{
			"zero value",
			[]Sample{
				Sample{Value: 0.0, Weight: 1.0},
			},
			NewSEHMetric("test"),
			SEHMetric{
				Name:  "test",
				Stats: StatisticSet{SampleCount: 1.0, Sum: 0.0, Min: 0.0, Max: 0.0},
				Buckets: map[int64]float64{
					ZeroBucket: 1.0,
				},
			},
			nil,
		},
		{
			"single positive value",
			[]Sample{
				Sample{Value: 20.0, Weight: 1.0},
			},
			NewSEHMetric("test"),
			SEHMetric{
				Name:  "test",
				Stats: StatisticSet{SampleCount: 1.0, Sum: 20.0, Min: 20.0, Max: 20.0},
				Buckets: map[int64]float64{
					31: 1.0, // (log 20 / log 1.1 == 31.4...)
				},
			},
			nil,
		},
		{
			"two values, one bucket",
			[]Sample{
				Sample{Value: 20.0, Weight: 1.0},
				Sample{Value: 21.0, Weight: 1.0},
			},
			NewSEHMetric("test"),
			SEHMetric{
				Name:  "test",
				Stats: StatisticSet{SampleCount: 2.0, Sum: 41.0, Min: 20.0, Max: 21.0},
				Buckets: map[int64]float64{
					31: 2.0, // (log 21 / log 1.1 == 31.9...)
				},
			},
			nil,
		},
		{
			"two values, two buckets",
			[]Sample{
				Sample{Value: 20.0, Weight: 1.0},
				Sample{Value: 50.0, Weight: 1.0},
			},
			NewSEHMetric("test"),
			SEHMetric{
				Name:  "test",
				Stats: StatisticSet{SampleCount: 2.0, Sum: 70.0, Min: 20.0, Max: 50.0},
				Buckets: map[int64]float64{
					31: 1.0, // (log 20 / log 1.1 == 31.4...)
					41: 1.0, // (log 50 / log 1.1 == 41.0...)
				},
			},
			nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := error(nil)
			for _, s := range c.samples {
				add_err := c.metric.AddWeightedSample(s.Value, s.Weight)
				if add_err != nil {
					err = add_err
				}
			}
			if err != c.expectedError {
				t.Errorf("expected %v, but received %v", err, c.expectedError)
			}

			if !reflect.DeepEqual(c.metric, c.expectedMetric) {
				t.Errorf("expected %v, but received %v", c.expectedMetric, c.metric)
			}
		})
	}
}
