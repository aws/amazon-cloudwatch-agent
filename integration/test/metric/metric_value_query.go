// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metric

import (
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go/aws"
	"log"
	"time"
)

func subtractMinutes(fromTime time.Time, minutes int) time.Time {
	tenMinutes := time.Duration(-1*minutes) * time.Minute
	return fromTime.Add(tenMinutes)
}

func FetchMetricValues() error {
	ec2InstanceId := test.GetInstanceId()
	instanceIdDimension := types.Dimension{
		Name:  aws.String("InstanceId"),
		Value: aws.String(ec2InstanceId),
	}
	cpuDimension := types.Dimension{
		Name:  aws.String("cpu"),
		Value: aws.String("cpu-total"),
	}
	dimensions := make([]types.Dimension, 2)
	dimensions[0] = instanceIdDimension
	dimensions[1] = cpuDimension

	metricToFetch := types.Metric{
		Namespace:  aws.String("MetricValueBenchmarkTest"),
		MetricName: aws.String("cpu_usage_active"),
		Dimensions: dimensions,
	}

	metricQueryPeriod := int32(60)

	metricQuery := types.MetricDataQuery{
		MetricStat: &types.MetricStat{
			Metric: &metricToFetch,
			Period: &metricQueryPeriod,
			Stat:   aws.String("Average"),
		},
		Id: aws.String("cpuUsageActive"),
	}

	metricDataQueries := make([]types.MetricDataQuery, 1)
	metricDataQueries[0] = metricQuery

	endTime := time.Now()
	startTime := subtractMinutes(endTime, 10)
	getMetricDataInput := cloudwatch.GetMetricDataInput{
		StartTime:         &startTime,
		EndTime:           &endTime,
		MetricDataQueries: metricDataQueries,
	}

	log.Printf("Metric data input is : %s", fmt.Sprint(getMetricDataInput))

	cwmClient, clientContext, err := test.GetCloudWatchMetricsClient()
	if err != nil {
		return fmt.Errorf("Error occurred while creating CloudWatch client: %v", err.Error())
	}

	output, err := cwmClient.GetMetricData(*clientContext, &getMetricDataInput)
	if err != nil {
		return fmt.Errorf("Error getting metric data %v", err)
	}

	log.Printf("Metric Value is : %s", fmt.Sprint(output.MetricDataResults[0].Values))
	return nil
}

// GetMetricDataInput = start time, end time,MetricDataQuery

// https://github.com/aws/aws-sdk-go-v2/blob/main/service/cloudwatch/api_op_GetMetricData.go

/*

func BuildDimensionFilterList(appendDimension int) []types.DimensionFilter {
	// we append dimension from 0 to max number - 2
	// then we add dimension instance id
	// thus for max dimension 10, 0 to 8 + instance id = 10 dimension
	ec2InstanceId := GetInstanceId()
	dimensionFilter := make([]types.DimensionFilter, appendDimension)
	for i := 0; i < appendDimension-1; i++ {
		dimensionFilter[i] = types.DimensionFilter{
			Name:  aws.String(fmt.Sprintf("%s%d", appendMetric, i)),
			Value: aws.String(fmt.Sprintf("%s%d", loremIpsum+appendMetric, i)),
		}
	}
	dimensionFilter[appendDimension-1] = types.DimensionFilter{
		Name:  aws.String(instanceId),
		Value: aws.String(ec2InstanceId),
	}
	return dimensionFilter
}

// ValidateMetrics takes the metric name, metric dimension and corresponding namespace that contains the metric
func ValidateMetrics(t *testing.T, metricName, namespace string, dimensionsFilter []types.DimensionFilter) {
	cwmClient, clientContext, err := GetCloudWatchMetricsClient()
	if err != nil {
		t.Fatalf("Error occurred while creating CloudWatch Logs SDK client: %v", err.Error())
	}

	listMetricsInput := cloudwatch.ListMetricsInput{
		MetricName:     aws.String(metricName),
		Namespace:      aws.String(namespace),
		RecentlyActive: "PT3H",
		Dimensions:     dimensionsFilter,
	}
	data, err := cwmClient.ListMetrics(*clientContext, &listMetricsInput)
	if err != nil {
		t.Errorf("Error getting metric data %v", err)
	}

	// Only validate if certain metrics are published by CloudWatchAgent in corresponding namespace
	// Since the metric value can be unpredictive.
	if len(data.Metrics) == 0 {
		metrics := make([]metric, len(dimensionsFilter))
		for i, filter := range dimensionsFilter {
			metrics[i] = metric{
				name:  *filter.Name,
				value: *filter.Value,
			}
		}
		t.Errorf("No metrics found for dimension %v metric name %v namespace %v",
			metrics, metricName, namespace)
	}

}
*/
