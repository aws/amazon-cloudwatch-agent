// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecs_metadata

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-cloudwatch-agent/integration/test"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

// Purpose: Detect the changes in metadata endpoint for ECS Container Agent https://github.com/aws/amazon-cloudwatch-agent/blob/master/translator/util/ecsutil/ecsutil.go#L67-L75
// Implementation: Checking if a log group's the format(https://github.com/aws/amazon-cloudwatch-agent/blob/master/translator/translate/logs/metrics_collected/prometheus/ruleLogGroupName.go#L33) 
// exists or not  since the log group's format has the scrapping cluster name from metadata endpoint.

const (
	RetryTime             = 10
	// Log group format: https://github.com/aws/amazon-cloudwatch-agent/blob/master/translator/translate/logs/metrics_collected/prometheus/ruleLogGroupName.go#L33
	ECSLogGroupNameFormat = "/aws/ecs/containerinsights/%s/prometheus" 
)

var clusterName = flag.String("clusterName", "", "Please provide the os preference, valid value: windows/linux.")

func TestValidatingCloudWatchLogs(t *testing.T) {

	ctx := context.Background()
	client := test.GetCWLogsClient(ctx)

	for currentRetry := 1; ; currentRetry++ {
		if currentRetry == RetryTime {
			t.Fatalf("Test metadata has exhausted %v retry time", RetryTime)
		}
		describeLogGroupInput := cloudwatchlogs.DescribeLogGroupsInput{
			LogGroupNamePrefix: aws.String(fmt.Sprintf(ECSLogGroupNameFormat, *clusterName)),
		}
		describeLogGroupOutput, err := client.DescribeLogGroups(ctx, &describeLogGroupInput)

		if err != nil {
			t.Errorf("Error getting log group data %v", err)
		}

		if len(describeLogGroupOutput.LogGroups) > 0 {
			break
		}

		fmt.Printf("Current retry: %v/%v and begin to sleep for 20s \n", currentRetry, RetryTime)
		time.Sleep(20 * time.Second)
	}
}
