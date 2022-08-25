// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build integration
// +build integration

package util

import (
	"context"
	"testing"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

var (
	metricsCtx context.Context
	cwm *cloudwatch.Client
)

// ValidateMetrics takes the metric name and corresponding namespace that holds the metric
// and return
func ValidateMetrics(t *testing.T, metricName, namespace string) {
	cwmClient, clientContext, err := getCloudWatchMetricsClient()
	if err != nil {
		t.Fatalf("Error occurred while creating CloudWatch Logs SDK client: %v", err.Error())
	}

	listMetricsInput := cloudwatch.ListMetricsInput{
		MetricName: aws.String(metricName),
		Namespace:  aws.String(namespace),
		RecentlyActive: "PT3H",
	}
	data, err := cwmClient.ListMetrics(*clientContext, &listMetricsInput)
	if err != nil {
		t.Errorf("Error getting metric data %v", err)
	}

	// Only validate if certain metrics are published by CloudWatchAgent in corresponding namespace
	// Since the metric value can be unpredictive.
	if len(data.Metrics) == 0 {
		t.Errorf("There are no metrics submitted by CloudWatchAgent recently")
	}

}

// getCloudWatchMetricsClient returns a singleton SDK client for interfacing with CloudWatch Metrics
func getCloudWatchMetricsClient() (*cloudwatch.Client, *context.Context, error) {
	if cwm == nil {
		metricsCtx = context.Background()
		c, err := config.LoadDefaultConfig(metricsCtx)
		if err != nil {
			return nil, nil, err
		}

		cwm = cloudwatch.NewFromConfig(c)
	}
	return cwm, &metricsCtx, nil
}
