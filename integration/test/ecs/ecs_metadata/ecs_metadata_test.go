// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT


package ecs_metadata

import (
	"flag"
	"context"
	"testing"
	"fmt"
	"time"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	
)

type RetriableError struct {
	Err        error
	RetryAfter time.Duration
}

const (
	RetryTime = 10
	ECSLogGroupNameFormat = "/aws/ecs/containerinsights/%s/prometheus"
)
var clusterName = flag.String("clusterName", "", "Please provide the os preference, valid value: windows/linux.")

func TestNumberMetricDimension(t *testing.T) {
	// test for cloud watch metrics
	ctx := context.Background()
	client := test.GetCWLogsClient(ctx)
	
	for currentRetry := 1; ; currentRetry++ {
		if currentRetry == RetryTime {
			t.Fatalf("Test metadata has exhausted %v retry time",RetryTime)
		}
		describeLogGroupInput := cloudwatchlogs.DescribeLogGroupsInput{
			LogGroupNamePrefix: aws.String(fmt.Sprintf(ECSLogGroupNameFormat, *clusterName)),
		}
		describeLogGroupOutput, err := client.DescribeLogGroups(ctx, &describeLogGroupInput)
		
		if err != nil {
			t.Errorf("Error getting metric data %v", err)
		}
		
		if len(describeLogGroupOutput.LogGroups) > 0 {
			break
		}
		
		fmt.Printf("Current retry: %v/%v and begin to sleep for 20s \n", currentRetry, RetryTime)
		time.Sleep(20 * time.Second)
	}
}