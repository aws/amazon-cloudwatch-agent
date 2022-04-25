// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package test

import (
	"context"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
)

var (
	ctx context.Context
	cwl *cloudwatchlogs.Client
)

// ValidateLogs takes a log group and log stream, and fetches the log events via the GetLogEvents
// API for all of the logs since a given timestamp, and checks if the number of log events matches
// the expected value.
func ValidateLogs(t *testing.T, logGroup, logStream string, numExpectedLogs int, since time.Time) {
	log.Printf("Checking %s/%s since %s for %d expected logs", logGroup, logStream, since.UTC().Format(time.RFC3339), numExpectedLogs)
	cwlClient, clientContext, err := getCloudWatchLogsClient()
	if err != nil {
		t.Fatalf("Error occurred while creating CloudWatch Logs SDK client: %v", err.Error())
	}

	sinceMs := since.UnixNano() / 1e6 // convert to millisecond timestamp

	// https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_GetLogEvents.html
	// GetLogEvents can return an empty result while still having more log events on a subsequent page,
	// so rather than expecting all the events to show up in one GetLogEvents API call, we need to paginate.
	params := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
		StartTime:     aws.Int64(sinceMs),
	}

	numLogsFound := 0
	var output *cloudwatchlogs.GetLogEventsOutput
	var nextToken *string

	for {
		if nextToken != nil {
			params.NextToken = nextToken
		}
		output, err = cwlClient.GetLogEvents(*clientContext, params)

		if err != nil {
			t.Fatalf("Error occurred while getting log events: %v", err.Error())
		}

		if nextToken != nil && output.NextForwardToken != nil && *output.NextForwardToken == *nextToken {
			// From the docs: If you have reached the end of the stream, it returns the same token you passed in.
			log.Printf("Done paginating log events for %s/%s and found %d logs", logGroup, logStream, numLogsFound)
			break
		}

		nextToken = output.NextForwardToken
		numLogsFound += len(output.Events)
	}

	// using assert.Len() prints out the whole splice of log events which bloats the test log
	assert.Equal(t, numExpectedLogs, numLogsFound)
}

// ValidateLogsInOrder takes a log group, log stream, a list of specific log lines and a timestamp.
// It should query the given log stream for log events, and then confirm that the log lines that are
// returned match the expected log lines.
func ValidateLogsInOrder(t *testing.T, logGroup, logStream string, logLines []string, since time.Time) {
	log.Printf("Checking %s/%s since %s for %d expected logs", logGroup, logStream, since.UTC().Format(time.RFC3339), len(logLines))
	cwlClient, clientContext, err := getCloudWatchLogsClient()
	if err != nil {
		t.Fatalf("Error occurred while creating CloudWatch Logs SDK client: %v", err.Error())
	}

	sinceMs := since.UnixNano() / 1e6 // convert to millisecond timestamp

	// https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_GetLogEvents.html
	// GetLogEvents can return an empty result while still having more log events on a subsequent page,
	// so rather than expecting all the events to show up in one GetLogEvents API call, we need to paginate.
	params := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
		StartTime:     aws.Int64(sinceMs),
		StartFromHead: aws.Bool(true), // read from the beginning
	}

	foundLogs := make([]string, len(logLines))
	var output *cloudwatchlogs.GetLogEventsOutput
	var nextToken *string

	for {
		if nextToken != nil {
			params.NextToken = nextToken
		}
		output, err = cwlClient.GetLogEvents(*clientContext, params)

		if err != nil {
			t.Fatalf("Error occurred while getting log events: %v", err.Error())
		}

		for _, e := range output.Events {
			foundLogs = append(foundLogs, *e.Message)
			log.Printf("Found log: %s", *e.Message)
		}

		if nextToken != nil && output.NextForwardToken != nil && *output.NextForwardToken == *nextToken {
			// From the docs: If you have reached the end of the stream, it returns the same token you passed in.
			log.Printf("Done paginating log events for %s/%s and found %d logs", logGroup, logStream, len(foundLogs))
			break
		}

		nextToken = output.NextForwardToken
	}

	// Validate that each of the logs are found, in order and in full.
	assert.Len(t, foundLogs, len(logLines))
	for i := 0; i < len(logLines); i++ {
		assert.Equal(t, logLines[i], foundLogs[i])
	}
}

// DeleteLogGroupAndStream cleans up a log group and stream by name. This gracefully handles
// ResourceNotFoundException errors from calling the APIs
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

// getCloudWatchLogsClient returns a singleton SDK client for interfacing with CloudWatch Logs
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
