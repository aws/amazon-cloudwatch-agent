// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package test

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
	"log"
	"testing"
	"time"
)

var (
	ctx context.Context
	cwl *cloudwatchlogs.Client
)

func ValidateLogs(t *testing.T, logGroup, logStream string, numExpectedLogs int, since time.Time) {
	log.Printf("Checking %s/%s since %v for %d expected logs", logGroup, logStream, since, numExpectedLogs)
	cwlClient, clientContext, err := getCloudWatchLogsClient()
	if err != nil {
		t.Fatalf("Error occurred while creating CloudWatch Logs SDK client: %v", err.Error())
	}

	sinceMs := since.UnixNano() / 1e6 // convert to millisecond timestamp
	events, err := cwlClient.GetLogEvents(*clientContext, &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
		StartTime:     aws.Int64(sinceMs),
	})

	if err != nil {
		t.Fatalf("Error occurred when calling GetLogEvents: %v", err.Error())
	}

	// using assert.Len() prints out the whole splice of log events which bloats the test log
	assert.Equal(t, numExpectedLogs, len(events.Events))
}

func DeleteLogGroupAndStream(logGroupName, logStreamName string) {
	cwlClient, clientContext, err := getCloudWatchLogsClient()
	if err != nil {
		log.Printf("Error occurred while creating CloudWatch Logs SDK client: %v", err)
		return // terminate gracefully so this alone doesn't cause integration test failures
	}

	// catch ResourceNotFoundException when deleting the log group and log stream, as these
	// are not useful exceptions to log errors on during cleanup
	var rnf *types.ResourceNotFoundException

	_, err = cwlClient.DeleteLogStream(*clientContext, &cloudwatchlogs.DeleteLogStreamInput{
		LogGroupName:  aws.String(logGroupName),
		LogStreamName: aws.String(logStreamName),
	})
	if err != nil && !errors.As(err, &rnf) {
		log.Printf("Error occurred while deleting log stream %s: %v", logStreamName, err)
	}

	_, err = cwlClient.DeleteLogGroup(*clientContext, &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: aws.String(logGroupName),
	})
	if err != nil && !errors.As(err, &rnf) {
		log.Printf("Error occurred while deleting log group %s: %v", logGroupName, err)
	}
}

func getCloudWatchLogsClient() (*cloudwatchlogs.Client, *context.Context, error) {
	if cwl == nil {
		ctx = context.Background()
		c, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, nil, err
		}

		cwl = cloudwatchlogs.NewFromConfig(c)
	}
	return cwl, &ctx, nil
}
