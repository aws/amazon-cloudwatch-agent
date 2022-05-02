// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build integration
// +build integration

package ecs_metadata

import (
	"flag"
	"fmt"
	"testing"
	"time"
	"github.com/aws/amazon-cloudwatch-agent/integration/test"
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
	logGroupName := fmt.Sprintf(ECSLogGroupNameFormat, *clusterName)

	for currentRetry := 1; ; currentRetry++ {

		if currentRetry == RetryTime {
			t.Fatalf("Test metadata has exhausted %v retry time", RetryTime)
		}

		if test.isLogGroupExists(t,logGroupName).(bool) {
			break
		}

		fmt.Printf("Current retry: %v/%v and begin to sleep for 20s \n", currentRetry, RetryTime)
		time.Sleep(20 * time.Second)
	}
}
