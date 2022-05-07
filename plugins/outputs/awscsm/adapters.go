// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscsm

import (
	"github.com/aws/amazon-cloudwatch-agent/awscsm/sdkmetricsdataplane"
	"github.com/aws/aws-sdk-go/aws"

	awscsmmetrics "github.com/aws/amazon-cloudwatch-agent/awscsm"
)

func adaptToCSMFrequencyMetrics(m []awscsmmetrics.FrequencyMetric) []*sdkmetricsdataplane.FrequencyMetric {
	metrics := []*sdkmetricsdataplane.FrequencyMetric{}

	for _, metric := range m {
		entries := []*sdkmetricsdataplane.FrequencyDistributionEntry{}

		for k, count := range metric.Frequencies {
			d := &sdkmetricsdataplane.FrequencyDistributionEntry{
				Key:   aws.String(k),
				Count: aws.Int64(count),
			}
			entries = append(entries, d)
		}

		csmMetric := &sdkmetricsdataplane.FrequencyMetric{
			Name:         aws.String(metric.Name),
			Distribution: entries,
		}

		metrics = append(metrics, csmMetric)
	}

	return metrics
}

func adaptToCSMSEHMetrics(m []awscsmmetrics.SEHMetric) []*sdkmetricsdataplane.SehMetric {
	metrics := []*sdkmetricsdataplane.SehMetric{}

	for _, metric := range m {
		buckets := []*sdkmetricsdataplane.SehBucket{}

		for k, count := range metric.Buckets {
			b := &sdkmetricsdataplane.SehBucket{
				Index: aws.Int64(k),
				Value: aws.Float64(count),
			}
			buckets = append(buckets, b)
		}

		sehStats := metric.Stats
		csmMetric := &sdkmetricsdataplane.SehMetric{
			Name:       aws.String(metric.Name),
			SehBuckets: buckets,
			Stats: &sdkmetricsdataplane.StatisticSet{
				Count: &sehStats.SampleCount,
				Sum:   &sehStats.Sum,
				Min:   &sehStats.Min,
				Max:   &sehStats.Max,
			},
		}

		metrics = append(metrics, csmMetric)
	}

	return metrics
}
