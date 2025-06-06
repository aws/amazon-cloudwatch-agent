// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-cloudwatch-agent/internal/logscommon"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

func TestTargetManager(t *testing.T) {
	logger := testutil.NewNopLogger()

	t.Run("CreateLogStream", func(t *testing.T) {
		target := Target{Group: "G", Stream: "S"}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil).Once()

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, mockService, tempDir)
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

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, mockService, tempDir)
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

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, mockService, tempDir)
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

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, mockService, tempDir)
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

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, mockService, tempDir)
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

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, mockService, tempDir)
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

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, mockService, tempDir)
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

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, mockService, tempDir)
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

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, mockService, tempDir)
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

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, mockService, tempDir)
		err := manager.InitTarget(target)
		assert.NoError(t, err)
		time.Sleep(30 * time.Second)
		mockService.AssertExpectations(t)
		assertCacheLen(t, manager, 1)
	})

	t.Run("SetRetentionPolicy/Negative", func(t *testing.T) {
		target := Target{Group: "G", Stream: "S", Retention: -1}

		mockService := new(mockLogsService)

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, mockService, tempDir)
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

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, service, tempDir)
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

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, service, tempDir)
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

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, mockService, tempDir)
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

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, mockService, tempDir)
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

		tempDir := t.TempDir()
		manager := NewTargetManager(logger, mockService, tempDir)
		err := manager.InitTarget(target)
		assert.NoError(t, err)

		time.Sleep(30 * time.Second)

		mockService.AssertExpectations(t)
		mockService.AssertNotCalled(t, "DescribeLogGroups")
		assertCacheLen(t, manager, 1)
	})
}

func TestTargetManagerWithTTL(t *testing.T) {
	logger := testutil.NewNopLogger()

	t.Run("TTLExpired_CallsAPI", func(t *testing.T) {
		tempDir := t.TempDir()
		target := Target{Group: "TestGroup", Stream: "TestStream", Retention: 7}

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

		manager := NewTargetManager(logger, mockService, tempDir)
		err := manager.InitTarget(target)
		assert.NoError(t, err)
		time.Sleep(200 * time.Millisecond)

		mockService.AssertExpectations(t)

		manager.Stop()
		time.Sleep(200 * time.Millisecond)

		ttlFilePath := filepath.Join(tempDir, logscommon.RetentionPolicyTTLFileName)
		content, err := os.ReadFile(ttlFilePath)
		assert.NoError(t, err)
		assert.Contains(t, string(content), escapeLogGroup(target.Group))
	})

	t.Run("TTLNotExpired_SkipsAPI", func(t *testing.T) {
		tempDir := t.TempDir()
		target := Target{Group: "TestGroup", Stream: "TestStream", Retention: 7}

		// Create a TTL file with a recent timestamp
		ttlFilePath := filepath.Join(tempDir, logscommon.RetentionPolicyTTLFileName)
		now := time.Now()
		content := escapeLogGroup(target.Group) + ":" + strconv.FormatInt(now.UnixMilli(), 10) + "\n"
		err := os.WriteFile(ttlFilePath, []byte(content), 0644) // nolint:gosec
		assert.NoError(t, err)

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil).Once()
		// DescribeLogGroups should not be called because TTL is not expired

		manager := NewTargetManager(logger, mockService, tempDir)
		err = manager.InitTarget(target)
		assert.NoError(t, err)
		time.Sleep(100 * time.Millisecond)

		manager.Stop()
		time.Sleep(100 * time.Millisecond)

		mockService.AssertExpectations(t)
		mockService.AssertNotCalled(t, "DescribeLogGroups")
		mockService.AssertNotCalled(t, "PutRetentionPolicy")

		_, err = os.Stat(ttlFilePath)
		assert.NoError(t, err)
	})

	t.Run("Stop_SavesTTLState", func(t *testing.T) {
		tempDir := t.TempDir()
		target := Target{Group: "TestGroup", Stream: "TestStream", Retention: 7}

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

		manager := NewTargetManager(logger, mockService, tempDir)
		err := manager.InitTarget(target)
		assert.NoError(t, err)
		time.Sleep(200 * time.Millisecond)

		manager.Stop()
		time.Sleep(200 * time.Millisecond)

		ttlFilePath := filepath.Join(tempDir, logscommon.RetentionPolicyTTLFileName)
		content, err := os.ReadFile(ttlFilePath)
		assert.NoError(t, err)
		assert.Contains(t, string(content), escapeLogGroup(target.Group))
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

func assertCacheLen(t *testing.T, manager TargetManager, count int) {
	t.Helper()
	tm := manager.(*targetManager)
	tm.mu.Lock()
	defer tm.mu.Unlock()
	assert.Len(t, tm.cache, count)
}
