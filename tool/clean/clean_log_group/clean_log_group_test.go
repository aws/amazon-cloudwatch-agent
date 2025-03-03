// main_test.go
package main

import (
	"context"
	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/stretchr/testify/mock"
)

// MockCloudWatchLogsClient is a stub for cloudwatchlogs.Client
type MockCloudWatchLogsClient struct {
	mock.Mock
}

var _ cloudwatchlogsClient = (*MockCloudWatchLogsClient)(nil)

func (m *MockCloudWatchLogsClient) DescribeLogGroups(ctx context.Context, input *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	args := m.Called(ctx, input, optFns)
	return args.Get(0).(*cloudwatchlogs.DescribeLogGroupsOutput), args.Error(1)
}

func (m *MockCloudWatchLogsClient) DeleteLogGroup(ctx context.Context, input *cloudwatchlogs.DeleteLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteLogGroupOutput, error) {
	args := m.Called(ctx, input, optFns)
	return args.Get(0).(*cloudwatchlogs.DeleteLogGroupOutput), args.Error(1)
}

func (m *MockCloudWatchLogsClient) DescribeLogStreams(ctx context.Context, input *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	args := m.Called(ctx, input, optFns)
	return args.Get(0).(*cloudwatchlogs.DescribeLogStreamsOutput), args.Error(1)
}

// Test getLastLogEventTime simulating multiple pages of log streams.
func TestGetLastLogEventTime(t *testing.T) {
	mockClient := new(MockCloudWatchLogsClient)

	// Create two pages of responses
	firstPage := &cloudwatchlogs.DescribeLogStreamsOutput{
		LogStreams: []types.LogStream{
			{LastEventTimestamp: aws.Int64(1000)},
			{LastEventTimestamp: aws.Int64(1500)},
		},
		NextToken: aws.String("token1"),
	}
	secondPage := &cloudwatchlogs.DescribeLogStreamsOutput{
		LogStreams: []types.LogStream{
			{LastEventTimestamp: aws.Int64(2000)},
			{LastEventTimestamp: aws.Int64(1800)},
		},
		NextToken: nil,
	}

	// Set up expectations for the two API calls
	mockClient.On("DescribeLogStreams",
		mock.Anything,
		&cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String("dummy-log-group"),
			OrderBy:      types.OrderByLastEventTime,
			Descending:   aws.Bool(true),
			NextToken:    nil,
		},
		mock.Anything).Return(firstPage, nil).Once()

	mockClient.On("DescribeLogStreams",
		mock.Anything,
		&cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String("dummy-log-group"),
			OrderBy:      types.OrderByLastEventTime,
			Descending:   aws.Bool(true),
			NextToken:    aws.String("token1"),
		},
		mock.Anything).Return(secondPage, nil).Once()

	lastEventTime := getLastLogEventTime(context.Background(), mockClient, "dummy-log-group")

	assert.Equal(t, int64(2000), lastEventTime)
	mockClient.AssertExpectations(t)
}
func testHandleLogGroup(cfg Config, logGroupName string, logCreationDate, logStreamCreationDate int) ([]string, error) {
	cfg.dryRun = true // Prevent actual deletion.

	// Calculate cutoffs relative to now.
	now := time.Now()
	times := cutoffTimes{
		creation: now.Add(cfg.creationThreshold).UnixMilli(),
		inactive: now.Add(cfg.inactiveThreshold).UnixMilli(),
	}
	// Create a dummy log group.
	creationTime := now.AddDate(0, 0, -logCreationDate).UnixMilli()
	logGroup := &types.LogGroup{
		LogGroupName: aws.String(logGroupName),
		CreationTime: aws.Int64(creationTime),
	}

	mockClient := new(MockCloudWatchLogsClient)

	// Set up expectation for DescribeLogStreams
	mockClient.On("DescribeLogStreams",
		mock.Anything,
		&cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String(logGroupName),
			OrderBy:      types.OrderByLastEventTime,
			Descending:   aws.Bool(true),
			NextToken:    nil,
		},
		mock.Anything).Return(&cloudwatchlogs.DescribeLogStreamsOutput{
		LogStreams: []types.LogStream{
			{LastEventTimestamp: aws.Int64(now.AddDate(0, 0, -logStreamCreationDate).UnixMilli())},
		},
		NextToken: nil,
	}, nil).Once()

	var deletedLogGroup []string
	var mutex sync.Mutex

	// Call handleLogGroup in dry-run mode (so no deletion call is made).
	err := handleLogGroup(context.Background(), mockClient, logGroup, &mutex, &deletedLogGroup, times, 1)
	return deletedLogGroup, err

}

// Test handleLogGroup to simulate deletion when a log group is old and inactive.
func TestHandleLogGroup(t *testing.T) {
	cfg := Config{
		creationThreshold: 3 * clean.KeepDurationOneDay,
		inactiveThreshold: 2 * clean.KeepDurationOneDay,
		numWorkers:        0,
		deleteBatchCap:    0,
		exceptionList:     []string{"EXCEPTION"},
		dryRun:            true,
	}
	testCases := []struct {
		name                  string
		logGroupName          string
		logCreationDate       int
		logStreamCreationDate int
		expected              []string
	}{
		{
			"Expired log group",
			"expired-test-log-group",
			7,
			7,
			[]string{"expired-test-log-group"},
		},
		{
			"Fresh log group",
			"fresh-test-log-group",
			1,
			7,
			[]string{},
		},
		{
			"Old but still used log group",
			"old-test-log-group",
			7,
			1,
			[]string{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.logGroupName, func(t *testing.T) {
			deletedLogGroup, err := testHandleLogGroup(cfg, tc.logGroupName, tc.logCreationDate, tc.logStreamCreationDate)
			assert.NoError(t, err)
			assert.Len(t, deletedLogGroup, len(tc.expected))
			assert.ElementsMatch(t, deletedLogGroup, tc.expected)
		})
	}

}
func testDeleteLogGroup(cfg Config, logGroupName string, logCreationDate, logStreamCreationDate int) []string {
	cfg.dryRun = true // Prevent actual deletion.
	now := time.Now()

	mockClient := new(MockCloudWatchLogsClient)

	// Set up expectation for DescribeLogGroups
	mockClient.On("DescribeLogGroups",
		mock.Anything,
		&cloudwatchlogs.DescribeLogGroupsInput{
			NextToken: nil,
		},
		mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{
		LogGroups: []types.LogGroup{
			{LogGroupName: aws.String(logGroupName),
				CreationTime: aws.Int64(now.AddDate(0, 0, -logCreationDate).UnixMilli())},
		},
		NextToken: nil,
	}, nil).Once()
	mockClient.On("DescribeLogStreams",
		mock.Anything,
		&cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String(logGroupName),
			OrderBy:      types.OrderByLastEventTime,
			Descending:   aws.Bool(true),
			NextToken:    nil,
		},
		mock.Anything).Return(&cloudwatchlogs.DescribeLogStreamsOutput{
		LogStreams: []types.LogStream{
			{LastEventTimestamp: aws.Int64(now.AddDate(0, 0, -logStreamCreationDate).UnixMilli())},
		},
		NextToken: nil,
	}, nil).Once()

	// Call handleLogGroup in dry-run mode (so no deletion call is made).
	return deleteOldLogGroups(context.Background(), mockClient, calculateCutoffTimes())

}
func TestDeleteLogGroups(t *testing.T) {
	cfg := Config{
		creationThreshold: 3,
		inactiveThreshold: 2,
		numWorkers:        0,
		deleteBatchCap:    0,
		exceptionList:     []string{"except"},
		dryRun:            true,
	}
	testCases := []struct {
		name                  string
		logGroupName          string
		logCreationDate       int
		logStreamCreationDate int
		expected              []string
	}{
		{
			"Expired log group",
			"expired-test-log-group",
			7,
			7,
			[]string{"expired-test-log-group"},
		},
		{
			"Fresh log group",
			"fresh-test-log-group",
			1,
			7,
			[]string{},
		},
		{
			"Old but still used log group",
			"old-test-log-group",
			7,
			1,
			[]string{},
		},
		{
			"Exception log group",
			"exceptional-test-log-group",
			7,
			1,
			[]string{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.logGroupName, func(t *testing.T) {
			deletedLogGroup := testDeleteLogGroup(cfg, tc.logGroupName, tc.logCreationDate, tc.logStreamCreationDate)
			assert.Len(t, deletedLogGroup, len(tc.expected))
			assert.ElementsMatch(t, deletedLogGroup, tc.expected)
		})
	}

}
