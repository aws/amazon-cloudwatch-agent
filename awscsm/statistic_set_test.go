package awscsmmetrics

import (
	"reflect"
	"testing"
)

func TestMerge(t *testing.T) {
	cases := []struct {
		testName             string
		initialDistribution  StatisticSet
		otherDistribution    StatisticSet
		expectedDistribution StatisticSet
		expectedError        error
	}{
		{
			"both empty",
			StatisticSet{},
			StatisticSet{},
			StatisticSet{},
			nil,
		},
		{
			"this has negative sample count",
			StatisticSet{ // garbage distribution
				SampleCount: -1.0,
				Sum:         1.0,
				Min:         2.0,
				Max:         3.0,
			},
			StatisticSet{ // good distribution
				SampleCount: 2.0,
				Sum:         5.0,
				Min:         2.0,
				Max:         3.0,
			},
			StatisticSet{ // expected is unchanged initial value
				SampleCount: -1.0,
				Sum:         1.0,
				Min:         2.0,
				Max:         3.0,
			},
			errNegativeSampleCount,
		},
		{
			"other has negative sample count",
			StatisticSet{ // good distribution
				SampleCount: 2.0,
				Sum:         5.0,
				Min:         2.0,
				Max:         3.0,
			},
			StatisticSet{ // garbage distribution
				SampleCount: -1.0,
				Sum:         -1.0,
				Min:         -2.0,
				Max:         -3.0,
			},
			StatisticSet{ // expected is unchanged initial value
				SampleCount: 2.0,
				Sum:         5.0,
				Min:         2.0,
				Max:         3.0,
			},
			errNegativeSampleCount,
		},
		{
			"this has zero sample count",
			StatisticSet{ // zero distribution
				SampleCount: 0.0,
				Sum:         1.0,
				Min:         2.0,
				Max:         3.0,
			},
			StatisticSet{ // good distribution
				SampleCount: 2.0,
				Sum:         5.0,
				Min:         2.0,
				Max:         3.0,
			},
			StatisticSet{ // expected is copy of good
				SampleCount: 2.0,
				Sum:         5.0,
				Min:         2.0,
				Max:         3.0,
			},
			nil,
		},
		{
			"other has zero sample count",
			StatisticSet{ // good distribution
				SampleCount: 2.0,
				Sum:         5.0,
				Min:         2.0,
				Max:         3.0,
			},
			StatisticSet{ // zero distribution
				SampleCount: 0.0,
				Sum:         -1.0,
				Min:         -2.0,
				Max:         -3.0,
			},
			StatisticSet{ // good remains unchanged
				SampleCount: 2.0,
				Sum:         5.0,
				Min:         2.0,
				Max:         3.0,
			},
			nil,
		},
		{
			"two good distributions",
			StatisticSet{ // good distribution 1
				SampleCount: 2.0,
				Sum:         5.0,
				Min:         2.0,
				Max:         3.0,
			},
			StatisticSet{ // good distribution 2
				SampleCount: 10.0,
				Sum:         -15.0,
				Min:         -40.0,
				Max:         5.0,
			},
			StatisticSet{ // good distribution
				SampleCount: 12.0,
				Sum:         -10.0,
				Min:         -40.0,
				Max:         5.0,
			},
			nil,
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			err := c.initialDistribution.Merge(c.otherDistribution)

			if err != c.expectedError {
				t.Errorf("expected %v, but received %v", err, c.expectedError)
			}

			if !reflect.DeepEqual(c.initialDistribution, c.expectedDistribution) {
				t.Errorf("expected %v, but received %v", c.expectedDistribution, c.initialDistribution)
			}

		})
	}
}
