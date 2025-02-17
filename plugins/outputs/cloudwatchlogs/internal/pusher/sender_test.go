// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

type mockLogsService struct {
	mock.Mock
}

func (m *mockLogsService) PutLogEvents(input *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.PutLogEventsOutput), args.Error(1)
}

func (m *mockLogsService) CreateLogStream(input *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.CreateLogStreamOutput), args.Error(1)
}

func (m *mockLogsService) CreateLogGroup(input *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.CreateLogGroupOutput), args.Error(1)
}

func (m *mockLogsService) PutRetentionPolicy(input *cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.PutRetentionPolicyOutput), args.Error(1)
}

func (m *mockLogsService) DescribeLogGroups(input *cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	args := m.Called(input)
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

	t.Run("Send/RejectedLogEvents", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		batch.append(newLogEvent(time.Now(), "Test message", nil))

		rejectedInfo := &cloudwatchlogs.RejectedLogEventsInfo{
			TooOldLogEventEndIndex:   aws.Int64(1),
			TooNewLogEventStartIndex: aws.Int64(2),
			ExpiredLogEventEndIndex:  aws.Int64(3),
		}

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		mockService.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{RejectedLogEventsInfo: rejectedInfo}, nil).Once()

		s := newSender(logger, mockService, mockManager, time.Second, make(chan struct{}))
		s.Send(batch)

		mockService.AssertExpectations(t)
	})

	t.Run("Send/ResourceNotFound", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		batch.append(newLogEvent(time.Now(), "Test message", nil))

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		mockService.On("PutLogEvents", mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, &cloudwatchlogs.ResourceNotFoundException{}).Twice()
		mockManager.On("InitTarget", mock.Anything).Return(errors.New("test")).Once()
		mockManager.On("InitTarget", mock.Anything).Return(nil).Once()
		mockService.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Once()

		s := newSender(logger, mockService, mockManager, time.Second, make(chan struct{}))
		s.Send(batch)

		mockService.AssertExpectations(t)
		mockManager.AssertExpectations(t)
	})

	t.Run("Error/InvalidParameter", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		batch.append(newLogEvent(time.Now(), "Test message", nil))

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		mockService.On("PutLogEvents", mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, &cloudwatchlogs.InvalidParameterException{}).Once()

		s := newSender(logger, mockService, mockManager, time.Second, make(chan struct{}))
		s.Send(batch)

		mockService.AssertExpectations(t)
	})

	t.Run("Error/DataAlreadyAccepted", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		batch.append(newLogEvent(time.Now(), "Test message", nil))

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		mockService.On("PutLogEvents", mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, &cloudwatchlogs.DataAlreadyAcceptedException{}).Once()

		s := newSender(logger, mockService, mockManager, time.Second, make(chan struct{}))
		s.Send(batch)

		mockService.AssertExpectations(t)
	})

	t.Run("Error/DropOnGeneric", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		batch.append(newLogEvent(time.Now(), "Test message", nil))

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		mockService.On("PutLogEvents", mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, errors.New("test")).Once()

		s := newSender(logger, mockService, mockManager, time.Second, make(chan struct{}))
		s.Send(batch)

		mockService.AssertExpectations(t)
	})

	t.Run("Error/RetryOnGenericAWS", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		batch.append(newLogEvent(time.Now(), "Test message", nil))

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		mockService.On("PutLogEvents", mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, awserr.New("SomeAWSError", "Some AWS error", nil)).Once()
		mockService.On("PutLogEvents", mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Once()

		s := newSender(logger, mockService, mockManager, time.Second, make(chan struct{}))
		s.Send(batch)

		mockService.AssertExpectations(t)
	})

	t.Run("DropOnRetryExhaustion", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		batch.append(newLogEvent(time.Now(), "Test message", nil))

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		mockService.On("PutLogEvents", mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, awserr.New("SomeAWSError", "Some AWS error", nil)).Once()

		s := newSender(logger, mockService, mockManager, 100*time.Millisecond, make(chan struct{}))
		s.Send(batch)

		mockService.AssertExpectations(t)
	})

	t.Run("StopChannelClosed", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		batch.append(newLogEvent(time.Now(), "Test message", nil))

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		mockService.On("PutLogEvents", mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, awserr.New("SomeAWSError", "Some AWS error", nil)).Once()

		stopCh := make(chan struct{})
		s := newSender(logger, mockService, mockManager, time.Second, stopCh)

		go func() {
			time.Sleep(50 * time.Millisecond)
			close(stopCh)
		}()

		s.Send(batch)

		mockService.AssertExpectations(t)
	})
}
