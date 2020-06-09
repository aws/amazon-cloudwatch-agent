package awscsmmetrics

import (
	"fmt"
	"math"
)

// Type for a metric backed by a sparse exponential histogram (Seh) distribution
type SEHMetric struct {
	Name    string
	Stats   StatisticSet
	Buckets map[int64]float64
}

// NewSEHMetric creates a new metric with an empty distribution
func NewSEHMetric(name string) SEHMetric {
	return SEHMetric{
		Name:    name,
		Stats:   StatisticSet{},
		Buckets: map[int64]float64{},
	}
}

var errNegativeSEHSampleValue = fmt.Errorf("Seh distribution sample cannot have a negative value")

const (
	ZeroBucket = int64(-32768)
)

var bucketFactor = math.Log(1.0 + 0.1)

// AddSample will add a sample to an SEH metric. The zero bucket is specified
// to be -32768 which should not be hit by math.Log call.
func (m *SEHMetric) AddSample(value float64) error {
	return m.AddWeightedSample(value, 1.0)
}

func (m *SEHMetric) AddWeightedSample(v float64, weight float64) error {

	// unsupported negative values
	if v < 0.0 {
		return errNegativeSEHSampleValue
	}

	err := m.Stats.Merge(NewWeightedStatisticSet(v, weight))
	if err != nil {
		return err
	}

	bucket := ZeroBucket
	if v > 0.0 {
		bucket = int64(math.Log(v) / bucketFactor)
	}

	if b, ok := m.Buckets[bucket]; ok {
		m.Buckets[bucket] = b + weight
	} else {
		m.Buckets[bucket] = weight
	}

	return nil
}

// SEHMetrics is a collection of metrics
type SEHMetrics map[string]SEHMetric
