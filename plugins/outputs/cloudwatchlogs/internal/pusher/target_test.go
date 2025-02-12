// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

func TestTargetManager(t *testing.T) {
	logger := testutil.Logger{Name: "test"}

	t.Run("CreateLogStream", func(t *testing.T) {
		target := Target{Group: "G", Stream: "S"}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil).Once()

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)

		assert.NoError(t, err)
		mockService.AssertExpectations(t)
	})

	t.Run("CreateLogGroupAndStream", func(t *testing.T) {
		target := Target{Group: "G", Stream: "S", Class: "newClass"}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).
			Return(&cloudwatchlogs.CreateLogStreamOutput{}, awserr.New(cloudwatchlogs.ErrCodeResourceNotFoundException, "Log group not found", nil)).Once()
		mockService.On("CreateLogGroup", mock.Anything).Return(&cloudwatchlogs.CreateLogGroupOutput{}, nil).Once()
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, &cloudwatchlogs.ResourceAlreadyExistsException{}).Once()

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)

		assert.NoError(t, err)
		mockService.AssertExpectations(t)
	})

	t.Run("CreateLogGroup/Error", func(t *testing.T) {
		target := Target{Group: "G", Stream: "S"}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).
			Return(&cloudwatchlogs.CreateLogStreamOutput{}, awserr.New(cloudwatchlogs.ErrCodeResourceNotFoundException, "Log group not found", nil)).Once()
		mockService.On("CreateLogGroup", mock.Anything).
			Return(&cloudwatchlogs.CreateLogGroupOutput{}, awserr.New("SomeAWSError", "Failed to create log group", nil)).Once()

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)

		assert.Error(t, err)
		mockService.AssertExpectations(t)
	})

	t.Run("SetRetentionPolicy", func(t *testing.T) {
		target := Target{Group: "G", Stream: "S", Retention: 7}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil).Once()
		mockService.On("DescribeLogGroups", mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []*cloudwatchlogs.LogGroup{
				{
					LogGroupName:    aws.String(target.Group),
					RetentionInDays: aws.Int64(0),
				},
			},
		}, nil).Once()
		mockService.On("PutRetentionPolicy", mock.Anything).Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, nil).Once()

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)
		assert.NoError(t, err)
		// Wait for async operations to complete
		time.Sleep(100 * time.Millisecond)
		mockService.AssertExpectations(t)
	})

	t.Run("SetRetentionPolicy/NoChange", func(t *testing.T) {
		target := Target{Group: "G", Stream: "S", Retention: 7}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil).Once()
		mockService.On("DescribeLogGroups", mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []*cloudwatchlogs.LogGroup{
				{
					LogGroupName:    aws.String(target.Group),
					RetentionInDays: aws.Int64(7),
				},
			},
		}, nil).Once()

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)
		assert.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
		mockService.AssertExpectations(t)
		mockService.AssertNotCalled(t, "PutRetentionPolicy")
	})

	t.Run("SetRetentionPolicy/LogGroupNotFound", func(t *testing.T) {
		t.Parallel()
		target := Target{Group: "G", Stream: "S", Retention: 7}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil).Once()
		mockService.On("DescribeLogGroups", mock.Anything).
			Return(&cloudwatchlogs.DescribeLogGroupsOutput{}, &cloudwatchlogs.ResourceNotFoundException{}).Times(numBackoffRetries)

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)
		assert.NoError(t, err)
		time.Sleep(30 * time.Second)
		mockService.AssertExpectations(t)
		mockService.AssertNotCalled(t, "PutRetentionPolicy")
	})

	t.Run("SetRetentionPolicy/Error", func(t *testing.T) {
		t.Parallel()
		target := Target{Group: "G", Stream: "S", Retention: 7}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil).Once()
		mockService.On("DescribeLogGroups", mock.Anything).Return(&cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: []*cloudwatchlogs.LogGroup{
				{
					LogGroupName:    aws.String(target.Group),
					RetentionInDays: aws.Int64(0),
				},
			},
		}, nil).Once()
		mockService.On("PutRetentionPolicy", mock.Anything).
			Return(&cloudwatchlogs.PutRetentionPolicyOutput{},
				awserr.New("SomeAWSError", "Failed to set retention policy", nil)).Times(numBackoffRetries)

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)
		assert.NoError(t, err)
		time.Sleep(30 * time.Second)
		mockService.AssertExpectations(t)
	})

	t.Run("SetRetentionPolicy/Negative", func(t *testing.T) {
		target := Target{Group: "G", Stream: "S", Retention: -1}

		mockService := new(mockLogsService)

		manager := NewTargetManager(logger, mockService)
		manager.PutRetentionPolicy(target)

		mockService.AssertNotCalled(t, "PutRetentionPolicy", mock.Anything)
	})

	t.Run("ConcurrentInit", func(t *testing.T) {
		targets := []Target{
			{Group: "G1", Stream: "S1"},
			{Group: "G2", Stream: "S2"},
		}

		var count atomic.Int32
		service := new(stubLogsService)
		service.cls = func(*cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
			time.Sleep(10 * time.Millisecond)
			count.Add(1)
			return &cloudwatchlogs.CreateLogStreamOutput{}, nil
		}

		manager := NewTargetManager(logger, service)
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := manager.InitTarget(targets[i%len(targets)])
				assert.NoError(t, err)
			}()
		}

		wg.Wait()
		assert.EqualValues(t, len(targets), count.Load())
	})

	t.Run("InitTarget/ZeroRetention", func(t *testing.T) {
		target := Target{Group: "G", Stream: "S", Retention: 0}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil).Once()

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)
		assert.NoError(t, err)

		mockService.AssertExpectations(t)
		mockService.AssertNotCalled(t, "DescribeLogGroups")
		mockService.AssertNotCalled(t, "PutRetentionPolicy")
	})

	t.Run("NewLogGroup/SetRetention", func(t *testing.T) {
		target := Target{Group: "G", Stream: "S", Retention: 7}

		mockService := new(mockLogsService)
		// fails with ResourceNotFound
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, awserr.New(cloudwatchlogs.ErrCodeResourceNotFoundException, "Log group not found", nil)).Once()
		mockService.On("CreateLogGroup", mock.Anything).Return(&cloudwatchlogs.CreateLogGroupOutput{}, nil).Once()
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil).Once()
		// should be called directly without DescribeLogGroups
		mockService.On("PutRetentionPolicy", mock.MatchedBy(func(input *cloudwatchlogs.PutRetentionPolicyInput) bool {
			return *input.LogGroupName == target.Group && *input.RetentionInDays == int64(target.Retention)
		})).Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, nil).Once()

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)
		assert.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
		mockService.AssertExpectations(t)
		mockService.AssertNotCalled(t, "DescribeLogGroups")
	})

	t.Run("NewLogGroup/RetentionError", func(t *testing.T) {
		t.Parallel()
		target := Target{Group: "G", Stream: "S", Retention: 7}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, awserr.New(cloudwatchlogs.ErrCodeResourceNotFoundException, "Log group not found", nil)).Once()
		mockService.On("CreateLogGroup", mock.Anything).Return(&cloudwatchlogs.CreateLogGroupOutput{}, nil).Once()
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil).Once()
		// fails but should retry
		mockService.On("PutRetentionPolicy", mock.Anything).Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, awserr.New("InternalError", "Internal error", nil)).Times(numBackoffRetries)

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)
		assert.NoError(t, err)

		time.Sleep(30 * time.Second)

		mockService.AssertExpectations(t)
		mockService.AssertNotCalled(t, "DescribeLogGroups")
	})
}

func TestCalculateBackoff(t *testing.T) {
	manager := &targetManager{}
	// should never exceed 30sec of total wait time
	totalDelay := time.Duration(0)
	for i := 0; i < numBackoffRetries; i++ {
		delay := manager.calculateBackoff(i)
		totalDelay += delay
	}
	assert.True(t, totalDelay <= 30*time.Second, "Total delay across all attempts should not exceed 30 seconds, but was %v", totalDelay)
}
