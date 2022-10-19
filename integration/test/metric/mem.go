//go:build linux && integration
// +build linux,integration

package metric

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"log"
)

var (
	memSupportedMetricValues = map[string]struct{}{
		"active":            {},
		"available":         {},
		"available_percent": {},
		"buffered":          {},
		"cached":            {},
		"free":              {},
		"inactive":          {},
		"total":             {},
		"used":              {},
		"used_percent":      {},
	}
	memMetricsSpecificDimension = []types.Dimension{
		{
			Name:  aws.String("mem"),
			Value: aws.String("mem-total"),
		},
	}
)

type MemMetricValueFetcher struct {
	baseMetricValueFetcher
}

var _ MetricValueFetcher = (*MemMetricValueFetcher)(nil)

func (f *MemMetricValueFetcher) Fetch(namespace, metricName string, stat Statistics) ([]float64, error) {
	dims := f.getMetricSpecificDimensions()
	values, err := f.fetch(namespace, metricName, dims, stat)
	if err != nil {
		log.Printf("Error while fetching metric value for %s: %v", metricName, err)
	}
	return values, err
}

func (f *MemMetricValueFetcher) isApplicable(metricName string) bool {
	_, exists := memSupportedMetricValues[metricName]
	return exists
}

func (f *MemMetricValueFetcher) getMetricSpecificDimensions() []types.Dimension {
	return memMetricsSpecificDimension
}
