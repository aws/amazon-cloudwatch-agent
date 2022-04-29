// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT


package ecs_metadata

import (
	"context"
	"testing"
	"fmt"
	
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
	
)
const namespace = "ECSMetadataTest"
const metricName = "disk_used_percent"

//Must run this test with parallel 1 since this will fail if more than one test is running at the same time
func TestNumberMetricDimension(t *testing.T) {


	// test for cloud watch metrics
	cxt := context.Background()
	client := test.GetCWClient(cxt)
	listMetricsInput := cloudwatch.ListMetricsInput{
		MetricName: aws.String(metricName),
		Namespace:  aws.String(namespace),
	}
	metrics, err := client.ListMetrics(cxt, &listMetricsInput)
	if err != nil {
		t.Errorf("Error getting metric data %v", err)
	}
	for _, metric := range metrics.Metrics {
		for _, dimension := range metric.Dimensions {
			fmt.Printf("%v %v \n",*metric.MetricName,*dimension.Name)
		}

	}

	

}

func getClusterInfo() (string, string) {
	ecsCluster := ecsutil.GetECSUtilSingleton()
	return ecsCluster.Region, ecsCluster.Cluster
	
}