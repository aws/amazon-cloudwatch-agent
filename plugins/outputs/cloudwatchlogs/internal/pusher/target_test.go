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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

func TestTargetManager(t *testing.T) {
	logger := testutil.NewNopLogger()

	t.Run("CreateLogStream", func(t *testing.T) {
		target := Target{Group: "G", Stream: "S"}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil).Once()

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)

		assert.NoError(t, err)
		mockService.AssertExpectations(t)
		assertCacheLen(t, manager, 1)
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
		assertCacheLen(t, manager, 1)
	})

	t.Run("CreateLogGroupAndStream/GroupAlreadyExists", func(t *testing.T) {
		target := Target{Group: "G1", Stream: "S1"}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).
			Return(&cloudwatchlogs.CreateLogStreamOutput{}, &cloudwatchlogs.ResourceNotFoundException{}).Once()
		mockService.On("CreateLogGroup", mock.Anything).Return(&cloudwatchlogs.CreateLogGroupOutput{}, &cloudwatchlogs.ResourceAlreadyExistsException{}).Once()
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil).Once()

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)

		assert.NoError(t, err)
		mockService.AssertExpectations(t)
		assertCacheLen(t, manager, 1)
	})

	t.Run("CreateLogGroupAndStream/RetryStreamFail", func(t *testing.T) {
		target := Target{Group: "G1", Stream: "S1"}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).
			Return(&cloudwatchlogs.CreateLogStreamOutput{}, &cloudwatchlogs.ResourceNotFoundException{}).Once()
		mockService.On("CreateLogGroup", mock.Anything).Return(&cloudwatchlogs.CreateLogGroupOutput{}, &cloudwatchlogs.ResourceAlreadyExistsException{}).Once()
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, &cloudwatchlogs.AccessDeniedException{}).Once()

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)

		assert.Error(t, err)
		mockService.AssertExpectations(t)
		assertCacheLen(t, manager, 0)
	})

	t.Run("CreateLogGroupAndStream/RetryStreamAlreadyExists", func(t *testing.T) {
		target := Target{Group: "G1", Stream: "S1"}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).
			Return(&cloudwatchlogs.CreateLogStreamOutput{}, &cloudwatchlogs.ResourceNotFoundException{}).Once()
		mockService.On("CreateLogGroup", mock.Anything).Return(&cloudwatchlogs.CreateLogGroupOutput{}, nil).Once()
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, &cloudwatchlogs.ResourceAlreadyExistsException{}).Once()

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)

		assert.NoError(t, err)
		mockService.AssertExpectations(t)
		assertCacheLen(t, manager, 1)
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
		assertCacheLen(t, manager, 0)
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
		assertCacheLen(t, manager, 1)
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
		assertCacheLen(t, manager, 1)
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
		assertCacheLen(t, manager, 1)
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
		assertCacheLen(t, manager, 1)
	})

	t.Run("SetRetentionPolicy/Negative", func(t *testing.T) {
		target := Target{Group: "G", Stream: "S", Retention: -1}

		mockService := new(mockLogsService)

		manager := NewTargetManager(logger, mockService)
		manager.PutRetentionPolicy(target)

		mockService.AssertNotCalled(t, "PutRetentionPolicy", mock.Anything)
		assertCacheLen(t, manager, 0)
	})

	t.Run("CreateLogGroup/Concurrent", func(t *testing.T) {
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
		assertCacheLen(t, manager, 2)
	})

	t.Run("CreateLogGroup/TTL", func(t *testing.T) {
		target := Target{Group: "G", Stream: "S"}

		var count atomic.Int32
		service := new(stubLogsService)
		service.cls = func(*cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
			count.Add(1)
			return &cloudwatchlogs.CreateLogStreamOutput{}, nil
		}

		manager := NewTargetManager(logger, service)
		manager.(*targetManager).cacheTTL = 50 * time.Millisecond
		for i := 0; i < 10; i++ {
			err := manager.InitTarget(target)
			assert.NoError(t, err)
		}
		assert.EqualValues(t, 1, count.Load())
		assertCacheLen(t, manager, 1)

		time.Sleep(50 * time.Millisecond)
		assertCacheLen(t, manager, 1)
		for i := 0; i < 10; i++ {
			err := manager.InitTarget(target)
			assert.NoError(t, err)
		}
		assert.EqualValues(t, 2, count.Load())
		assertCacheLen(t, manager, 1)
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
		assertCacheLen(t, manager, 1)
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
		assertCacheLen(t, manager, 1)
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
		assertCacheLen(t, manager, 1)
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

func TestTargetManager_RateLimiter(t *testing.T) {
	logger := testutil.NewNopLogger()

	t.Run("DescribeLogGroupsRateLimited", func(t *testing.T) {
		targets := []Target{
			{Group: "G1", Stream: "S1", Retention: 7},
			{Group: "G2", Stream: "S2", Retention: 14},
			{Group: "G3", Stream: "S3", Retention: 30},
		}

		service := new(stubLogsService)
		var mu sync.RWMutex
		var callTimes []time.Time

		service.cls = func(*cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
			return &cloudwatchlogs.CreateLogStreamOutput{}, nil
		}
		service.dlg = func(*cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			mu.Lock()
			callTimes = append(callTimes, time.Now())
			mu.Unlock()
			return &cloudwatchlogs.DescribeLogGroupsOutput{
				LogGroups: []*cloudwatchlogs.LogGroup{
					{
						LogGroupName:    aws.String("G1"),
						RetentionInDays: aws.Int64(0),
					},
					{
						LogGroupName:    aws.String("G2"),
						RetentionInDays: aws.Int64(0),
					},
					{
						LogGroupName:    aws.String("G3"),
						RetentionInDays: aws.Int64(0),
					},
				},
			}, nil
		}
		service.prp = func(*cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
			return &cloudwatchlogs.PutRetentionPolicyOutput{}, nil
		}

		manager := NewTargetManager(logger, service)
		var wg sync.WaitGroup
		for i := 0; i < len(targets); i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				err := manager.InitTarget(targets[idx])
				assert.NoError(t, err)
			}(i)
		}
		wg.Wait()

		assertCacheLen(t, manager, 3)

		// Check that there was at least 1 second between calls (rate limit is 1 per second)
		mu.RLock()
		callTimesCopy := make([]time.Time, len(callTimes)) // Make a copy to avoid race
		copy(callTimesCopy, callTimes)
		mu.RUnlock()

		for i := 1; i < len(callTimesCopy); i++ {
			timeDiff := callTimesCopy[i].Sub(callTimesCopy[i-1])
			assert.GreaterOrEqual(t, timeDiff, 995*time.Millisecond,
				"Expected at least ~1 second between DescribeLogGroups calls due to rate limiting: got %v", timeDiff)
		}
	})

	t.Run("PutRetentionPolicyRateLimited", func(t *testing.T) {
		targets := []Target{
			{Group: "G1", Stream: "S1", Retention: 7},
			{Group: "G2", Stream: "S2", Retention: 14},
			{Group: "G3", Stream: "S3", Retention: 30},
		}

		service := new(stubLogsService)
		var mu sync.RWMutex
		var callTimes []time.Time

		service.cls = func(*cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
			return &cloudwatchlogs.CreateLogStreamOutput{}, nil
		}
		service.dlg = func(*cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{
				LogGroups: []*cloudwatchlogs.LogGroup{
					{
						LogGroupName:    aws.String("G1"),
						RetentionInDays: aws.Int64(0),
					},
					{
						LogGroupName:    aws.String("G2"),
						RetentionInDays: aws.Int64(0),
					},
					{
						LogGroupName:    aws.String("G3"),
						RetentionInDays: aws.Int64(0),
					},
				},
			}, nil
		}
		service.prp = func(*cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
			mu.Lock()
			callTimes = append(callTimes, time.Now())
			mu.Unlock()
			return &cloudwatchlogs.PutRetentionPolicyOutput{}, nil
		}

		manager := NewTargetManager(logger, service)
		var wg sync.WaitGroup
		for i := 0; i < len(targets); i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				err := manager.InitTarget(targets[idx])
				assert.NoError(t, err)
			}(i)
		}
		wg.Wait()

		assertCacheLen(t, manager, 3)

		// Check that there was at least 1 second between calls (rate limit is 1 per second)
		mu.RLock()
		callTimesCopy := make([]time.Time, len(callTimes)) // Make a copy to avoid race
		copy(callTimesCopy, callTimes)
		mu.RUnlock()

		for i := 1; i < len(callTimesCopy); i++ {
			timeDiff := callTimesCopy[i].Sub(callTimesCopy[i-1])
			assert.GreaterOrEqual(t, timeDiff, 995*time.Millisecond,
				"Expected at least ~1 second between PutRetentionPolicy calls due to rate limiting: got %v", timeDiff)
		}
	})
}

func assertCacheLen(t *testing.T, manager TargetManager, count int) {
	t.Helper()
	tm := manager.(*targetManager)
	tm.mu.Lock()
	defer tm.mu.Unlock()
	assert.Len(t, tm.cache, count)
}
