// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build integration
// +build integration

package utils

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

var (
	ctx context.Context
	cwm *cloudwatchlogs.Client
)

func GetCWClient(cxt context.Context) *cloudwatch.Client {
	defaultConfig, err := config.LoadDefaultConfig(cxt)

	if err != nil {
		log.Fatalf("err occurred while creating config %v", err)
	}
	return cloudwatch.NewFromConfig(defaultConfig)
}

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
	data, err := cwmClient.ListMetrics(cxt, &listMetricsInput)
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
func getCloudWatchMetricsClient() (*cloudwatchlogs.Client, *context.Context, error) {
	if cwl == nil {
		ctx = context.Background()
		c, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, nil, err
		}

		cwm = cloudwatchlogs.NewFromConfig(c)
	}
	return cwm, &ctx, nil
}
