// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/influxdata/telegraf"
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

	// Errors that are not smithy.APIError must still be retried. These mirror the shapes the SDK v2 client
	// actually returns for transport, credential, and response-parsing failures.
	retryableErrors := map[string]error{
		"Generic": errors.New("test"),
		"ConnectionRefused": &smithy.OperationError{
			ServiceID:     "CloudWatch Logs",
			OperationName: "PutLogEvents",
			Err: &retry.MaxAttemptsError{
				Attempt: 3,
				Err: &smithyhttp.ResponseError{
					Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 0}},
					Err: &smithyhttp.RequestSendError{
						Err: &url.Error{Op: "Post", URL: "https://logs.us-west-2.amazonaws.com/", Err: &net.OpError{
							Op: "dial", Net: "tcp", Err: &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED},
						}},
					},
				},
			},
		},
		"DNSFailure": &smithy.OperationError{
			ServiceID:     "CloudWatch Logs",
			OperationName: "PutLogEvents",
			Err: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 0}},
				Err: &smithyhttp.RequestSendError{
					Err: &url.Error{Op: "Post", URL: "https://logs.us-west-2.amazonaws.com/", Err: &net.OpError{
						Op: "dial", Net: "tcp", Err: &net.DNSError{Err: "no such host", Name: "logs.us-west-2.amazonaws.com", IsNotFound: true},
					}},
				},
			},
		},
		"ClientTimeout": &smithy.OperationError{
			ServiceID:     "CloudWatch Logs",
			OperationName: "PutLogEvents",
			Err: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 0}},
				Err:      fmt.Errorf("canceled, %w", context.DeadlineExceeded),
			},
		},
		"CredentialsUnavailable": &smithy.OperationError{
			ServiceID:     "CloudWatch Logs",
			OperationName: "PutLogEvents",
			Err: fmt.Errorf("get identity: %w", fmt.Errorf("get credentials: %w",
				fmt.Errorf("failed to refresh cached credentials, %w", errors.New("no EC2 IMDS role found")))),
		},
		"MalformedResponseBody": &smithy.OperationError{
			ServiceID:     "CloudWatch Logs",
			OperationName: "PutLogEvents",
			Err: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 500}},
				Err: &smithy.DeserializationError{
					Err: errors.New("failed to decode response body, invalid character '<' looking for beginning of value"),
				},
			},
		},
	}
	for name, err := range retryableErrors {
		t.Run("Error/RetryOn"+name, func(t *testing.T) {
			assertRetriedThenSucceeds(t, logger, err)
		})
	}

	// Client-side errors that can never succeed on retry must be dropped without a second attempt.
	terminalErrors := map[string]error{
		"InvalidParams": &smithy.OperationError{
			ServiceID:     "CloudWatch Logs",
			OperationName: "PutLogEvents",
			Err:           smithy.InvalidParamsError{Context: "PutLogEventsInput"},
		},
		"Serialization": &smithy.OperationError{
			ServiceID:     "CloudWatch Logs",
			OperationName: "PutLogEvents",
			Err:           &smithy.SerializationError{Err: errors.New("failed to encode request")},
		},
	}
	for name, err := range terminalErrors {
		t.Run("Error/DropOn"+name, func(t *testing.T) {
			assertDroppedWithoutRetry(t, logger, err)
		})
	}

	t.Run("Error/RetryOnGenericAWS", func(t *testing.T) {
		batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)
		batch.append(newLogEvent(time.Now(), "Test message", nil))

		mockService := new(mockLogsService)
		mockManager := new(mockTargetManager)
		mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, &smithy.GenericAPIError{Code: "SomeAWSError", Message: "Some AWS error"}).Once()
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
		mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, &smithy.GenericAPIError{Code: "SomeAWSError", Message: "Some AWS error"}).Once()

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
		mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).
			Return(&cloudwatchlogs.PutLogEventsOutput{}, &smithy.GenericAPIError{Code: "SomeAWSError", Message: "Some AWS error"}).Once()

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

// assertRetriedThenSucceeds sends a batch whose first PutLogEvents fails with err and second succeeds, and asserts
// that the sender retried: two calls, done callback run (success), state callback run exactly once (by done).
func assertRetriedThenSucceeds(t *testing.T, logger telegraf.Logger, err error) {
	t.Helper()
	batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

	doneCallbackCalled := false
	batch.append(newLogEvent(time.Now(), "Test message", func() {
		doneCallbackCalled = true
	}))

	stateCallbackCount := 0
	batch.addStateCallback(func() {
		stateCallbackCount++
	})

	mockService := new(mockLogsService)
	mockManager := new(mockTargetManager)
	mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).
		Return(&cloudwatchlogs.PutLogEventsOutput{}, err).Once()
	mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).
		Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Once()

	s := newSender(logger, mockService, mockManager, 5*time.Second)
	s.Send(batch)
	s.Stop()

	mockService.AssertExpectations(t)
	mockService.AssertNumberOfCalls(t, "PutLogEvents", 2)
	assert.True(t, doneCallbackCalled, "Done callback should be called after a successful retry")
	assert.Equal(t, 1, stateCallbackCount, "State callback should run exactly once, on success")
}

// assertDroppedWithoutRetry sends a batch whose PutLogEvents fails with err and asserts that the sender gave up
// immediately: one call, state callback run (offset advanced), done callback not run.
func assertDroppedWithoutRetry(t *testing.T, logger telegraf.Logger, err error) {
	t.Helper()
	batch := newLogEventBatch(Target{Group: "G", Stream: "S"}, nil)

	doneCallbackCalled := false
	batch.append(newLogEvent(time.Now(), "Test message", func() {
		doneCallbackCalled = true
	}))

	stateCallbackCalled := false
	batch.addStateCallback(func() {
		stateCallbackCalled = true
	})

	mockService := new(mockLogsService)
	mockManager := new(mockTargetManager)
	mockService.On("PutLogEvents", mock.Anything, mock.Anything, mock.Anything).
		Return(&cloudwatchlogs.PutLogEventsOutput{}, err).Once()

	s := newSender(logger, mockService, mockManager, 5*time.Second)
	s.Send(batch)
	s.Stop()

	mockService.AssertExpectations(t)
	mockService.AssertNumberOfCalls(t, "PutLogEvents", 1)
	assert.True(t, stateCallbackCalled, "State callback should be called for a terminal error")
	assert.False(t, doneCallbackCalled, "Done callback should not be called for a terminal error")
}
