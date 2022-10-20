// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metric

import (
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"log"
)

var memSupportedMetricValues = map[string]struct{}{
	"mem_active":            {},
	"mem_available":         {},
	"mem_available_percent": {},
	"mem_buffered":          {},
	"mem_cached":            {},
	"mem_free":              {},
	"mem_inactive":          {},
	"mem_total":             {},
	"mem_used":              {},
	"mem_used_percent":      {},
}

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
	return []types.Dimension{}
}
