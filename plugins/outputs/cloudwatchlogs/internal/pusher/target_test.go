// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
		mockService.On("PutRetentionPolicy", mock.Anything).Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, nil).Once()

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)

		assert.NoError(t, err)
		mockService.AssertExpectations(t)
	})

	t.Run("SetRetentionPolicy/LogGroupNotFound", func(t *testing.T) {
		target := Target{Group: "G", Stream: "S", Retention: 7}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil).Once()
		mockService.On("PutRetentionPolicy", mock.Anything).
			Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, &cloudwatchlogs.ResourceNotFoundException{}).Once()

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)

		assert.NoError(t, err) // The overall operation should still succeed even if setting retention policy fails
		mockService.AssertExpectations(t)
	})

	t.Run("SetRetentionPolicy/Error", func(t *testing.T) {
		target := Target{Group: "G", Stream: "S", Retention: 7}

		mockService := new(mockLogsService)
		mockService.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil).Once()
		mockService.On("PutRetentionPolicy", mock.Anything).
			Return(&cloudwatchlogs.PutRetentionPolicyOutput{}, awserr.New("SomeAWSError", "Failed to set retention policy", nil)).Once()

		manager := NewTargetManager(logger, mockService)
		err := manager.InitTarget(target)

		assert.NoError(t, err) // The overall operation should still succeed even if setting retention policy fails
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
		target := Target{Group: "G", Stream: "S"}

		var count atomic.Int32
		service := new(stubLogsService)
		service.cls = func(*cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
			time.Sleep(10 * time.Millisecond)
			count.Add(1)
			return &cloudwatchlogs.CreateLogStreamOutput{}, nil
		}

		manager := NewTargetManager(logger, service)
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := manager.InitTarget(target)
				assert.NoError(t, err)
			}()
		}

		wg.Wait()
		assert.Equal(t, int32(1), count.Load())
	})
}
