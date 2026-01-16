// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

type mockLogsService struct {
	mock.Mock
}

func (m *mockLogsService) PutLogEvents(ctx context.Context, input *cloudwatchlogs.PutLogEventsInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*cloudwatchlogs.PutLogEventsOutput), args.Error(1)
}

func (m *mockLogsService) CreateLogStream(ctx context.Context, input *cloudwatchlogs.CreateLogStreamInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*cloudwatchlogs.CreateLogStreamOutput), args.Error(1)
}

func (m *mockLogsService) CreateLogGroup(ctx context.Context, input *cloudwatchlogs.CreateLogGroupInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*cloudwatchlogs.CreateLogGroupOutput), args.Error(1)
}

func (m *mockLogsService) PutRetentionPolicy(ctx context.Context, input *cloudwatchlogs.PutRetentionPolicyInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*cloudwatchlogs.PutRetentionPolicyOutput), args.Error(1)
}

func (m *mockLogsService) DescribeLogGroups(ctx context.Context, input *cloudwatchlogs.DescribeLogGroupsInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*cloudwatchlogs.DescribeLogGroupsOutput), args.Error(1)
}

type mockTargetManager struct {
	mock.Mock
}

func (m *mockTargetManager) InitTarget(target Target) error {
	args := m.Called(target)
	return args.Error(0)
}

func (m *mockTargetManager) PutRetentionPolicy(target Target) {
	m.Called(target)
}

func TestSender(t *testing.T) {
	logger := testutil.NewNopLogger()

	t.Run("Send/Success", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		doneCallbackCalled := false
		doneCallback := func() {
			doneCallbackCalled = true
		}
		batch.append(newLogEvent(time.Now(), "Test message", doneCallback))

		stateCallbackCalled := false
		batch.addStateCallback(func() {
			stateCallbackCalled = true
		})

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Once()

		s := newSender(logger, mockService, mockManager, time.Second)
		s.Send(batch)
		s.Stop()

		mockService.AssertExpectations(t)
		assert.True(t, stateCallbackCalled, "State callback was not called in success scenario")
		assert.True(t, doneCallbackCalled, "Done callback was not called in success scenario")
	})

	t.Run("Send/RejectedLogEvents", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		batch.append(newLogEvent(time.Now(), "Test message", nil))

		rejectedInfo := &types.RejectedLogEventsInfo{
			TooOldLogEventEndIndex:   aws.Int32(1),
			TooNewLogEventStartIndex: aws.Int32(2),
			ExpiredLogEventEndIndex:  aws.Int32(3),
		}

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{RejectedLogEventsInfo: rejectedInfo}, nil).Once()

		s := newSender(logger, mockService, mockManager, time.Second)
		s.Send(batch)
		s.Stop()

		mockService.AssertExpectations(t)
	})

	t.Run("Send/ResourceNotFound", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		batch.append(newLogEvent(time.Now(), "Test message", nil))

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, &types.ResourceNotFoundException{}).Twice()
		mockManager.On("InitTarget", mock.Anything).Return(errors.New("test")).Once()
		mockManager.On("InitTarget", mock.Anything).Return(nil).Once()
		mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Once()

		s := newSender(logger, mockService, mockManager, time.Second)
		s.Send(batch)
		s.Stop()

		mockService.AssertExpectations(t)
		mockManager.AssertExpectations(t)
	})

	t.Run("Error/InvalidParameter", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		doneCallbackCalled := false
		doneCallback := func() {
			doneCallbackCalled = true
		}
		batch.append(newLogEvent(time.Now(), "Test message", doneCallback))

		stateCallbackCalled := false
		batch.addStateCallback(func() {
			stateCallbackCalled = true
		})

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, &types.InvalidParameterException{}).Once()

		s := newSender(logger, mockService, mockManager, time.Second)
		s.Send(batch)
		s.Stop()

		mockService.AssertExpectations(t)
		assert.True(t, stateCallbackCalled, "State callback was not called for InvalidParameterException")
		assert.False(t, doneCallbackCalled, "Done callback should not be called for InvalidParameterException")
	})

	t.Run("Error/DataAlreadyAccepted", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		doneCallbackCalled := false
		doneCallback := func() {
			doneCallbackCalled = true
		}
		batch.append(newLogEvent(time.Now(), "Test message", doneCallback))

		stateCallbackCalled := false
		batch.addStateCallback(func() {
			stateCallbackCalled = true
		})

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, &types.DataAlreadyAcceptedException{}).Once()

		s := newSender(logger, mockService, mockManager, time.Second)
		s.Send(batch)
		s.Stop()

		mockService.AssertExpectations(t)
		assert.True(t, stateCallbackCalled, "State callback was not called for DataAlreadyAcceptedException")
		assert.False(t, doneCallbackCalled, "Done callback should not be called for DataAlreadyAcceptedException")
	})

	t.Run("Error/DropOnGeneric", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		doneCallbackCalled := false
		doneCallback := func() {
			doneCallbackCalled = true
		}
		batch.append(newLogEvent(time.Now(), "Test message", doneCallback))

		stateCallbackCalled := false
		batch.addStateCallback(func() {
			stateCallbackCalled = true
		})

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, errors.New("test")).Once()

		s := newSender(logger, mockService, mockManager, time.Second)
		s.Send(batch)
		s.Stop()

		mockService.AssertExpectations(t)
		assert.True(t, stateCallbackCalled, "State callback was not called for non-AWS error")
		assert.False(t, doneCallbackCalled, "Done callback should not be called for non-AWS error")
	})

	t.Run("Error/RetryOnGenericAWS", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		batch.append(newLogEvent(time.Now(), "Test message", nil))

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		// Create a generic AWS API error using smithy.GenericAPIError
		apiErr := &smithy.GenericAPIError{Code: "SomeAWSError", Message: "Some AWS error"}
		mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, apiErr).Once()
		mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Once()

		s := newSender(logger, mockService, mockManager, time.Second)
		s.Send(batch)
		s.Stop()

		mockService.AssertExpectations(t)
	})

	t.Run("DropOnRetryExhaustion", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		doneCallbackCalled := false
		doneCallback := func() {
			doneCallbackCalled = true
		}
		batch.append(newLogEvent(time.Now(), "Test message", doneCallback))

		stateCallbackCalled := false
		batch.addStateCallback(func() {
			stateCallbackCalled = true
		})

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		// Create a generic AWS API error using smithy.GenericAPIError
		apiErr := &smithy.GenericAPIError{Code: "SomeAWSError", Message: "Some AWS error"}
		mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, apiErr).Once()

		s := newSender(logger, mockService, mockManager, 100*time.Millisecond)
		s.Send(batch)
		s.Stop()

		mockService.AssertExpectations(t)
		assert.True(t, stateCallbackCalled, "State callback was not called when retry attempts were exhausted")
		assert.False(t, doneCallbackCalled, "Done callback should not be called when retry attempts are exhausted")
	})

	t.Run("StopChannelClosed", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

		doneCallbackCalled := false
		doneCallback := func() {
			doneCallbackCalled = true
		}
		batch.append(newLogEvent(time.Now(), "Test message", doneCallback))

		stateCallbackCalled := false
		batch.addStateCallback(func() {
			stateCallbackCalled = true
		})

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		// Create a generic AWS API error using smithy.GenericAPIError
		apiErr := &smithy.GenericAPIError{Code: "SomeAWSError", Message: "Some AWS error"}
		mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, apiErr).Once()

		s := newSender(logger, mockService, mockManager, time.Second)

		go func() {
			time.Sleep(50 * time.Millisecond)
			s.Stop()
		}()

		s.Send(batch)

		mockService.AssertExpectations(t)
		assert.True(t, stateCallbackCalled, "State callback was not called when stop was requested")
		assert.False(t, doneCallbackCalled, "Done callback should not be called when stop was requested")
	})
}
