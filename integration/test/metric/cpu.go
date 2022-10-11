// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metric

import (
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go/aws"
	"log"
)

type CPUMetricValueFetcher struct {
	base *baseMetricValueFetcher
}

func (f *CPUMetricValueFetcher) Fetch(namespace string, metricName string, stat Statistics) ([]float64, error) {
	dimensions := f.getMetricSpecificDimensions()
	values, err := f.fetch(namespace, dimensions, metricName, stat)
	if err != nil {
		log.Printf("Error while fetching metric value for %v: %v", metricName, err.Error())
	}
	return values, err
}

func (f *CPUMetricValueFetcher) fetch(namespace string, metricSpecificDimensions []types.Dimension, metricName string, stat Statistics) ([]float64, error) {
	return f.base.fetch(namespace, metricSpecificDimensions, metricName, stat)
}

var cpuSupportedMetricValues = map[string]struct{}{
	"cpu_time_active":      {},
	"cpu_time_guest":       {},
	"cpu_time_guest_nice":  {},
	"cpu_time_idle":        {},
	"cpu_time_iowait":      {},
	"cpu_time_irq":         {},
	"cpu_time_nice":        {},
	"cpu_time_softirq":     {},
	"cpu_time_steal":       {},
	"cpu_time_system":      {},
	"cpu_time_user":        {},
	"cpu_usage_active":     {},
	"cpu_usage_quest":      {},
	"cpu_usage_quest_nice": {},
	"cpu_usage_idle":       {},
	"cpu_usage_iowait":     {},
	"cpu_usage_irq":        {},
	"cpu_usage_nice":       {},
	"cpu_usage_softirq":    {},
	"cpu_usage_steal":      {},
	"cpu_usage_system":     {},
	"cpu_usage_user":       {},
}

func (f *CPUMetricValueFetcher) isApplicable(metricName string) bool {
	_, exists := cpuSupportedMetricValues[metricName]
	return exists
}

func (f *CPUMetricValueFetcher) getMetricSpecificDimensions() []types.Dimension {
	cpuDimension := types.Dimension{
		Name:  aws.String("cpu"),
		Value: aws.String("cpu-total"),
	}
	dimensions := make([]types.Dimension, 1)
	dimensions[0] = cpuDimension

	return dimensions
}
