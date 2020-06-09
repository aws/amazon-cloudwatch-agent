package awscsm

import (
	"testing"
	"time"
)

func TestFilterPrior(t *testing.T) {
	cases := []struct {
		filter   filterPrior
		record   AggregationRecord
		expected bool
	}{
		{
			filter: filterPrior{
				cutoff: time.Unix(10, 0),
			},
			record: AggregationRecord{
				Expiry: time.Unix(11, 0),
			},
			expected: true,
		},
		{
			filter: filterPrior{
				cutoff: time.Unix(10, 0),
			},
			record: AggregationRecord{
				Expiry: time.Unix(0, 0),
			},
			expected: false,
		},
	}

	for _, c := range cases {
		if e, a := c.expected, c.filter.Filter(c.record); e != a {
			t.Errorf("expected %t, but received %t", e, a)
		}
	}
}
