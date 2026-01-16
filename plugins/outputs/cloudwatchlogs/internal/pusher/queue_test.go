// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/smithy-go"
	"github.com/influxdata/telegraf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/state"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type stubLogsService struct {
	ple func(context.Context, *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error)
	clg func(context.Context, *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error)
	cls func(context.Context, *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error)
	prp func(context.Context, *cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error)
	dlg func(context.Context, *cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
}

func (s *stubLogsService) PutLogEvents(ctx context.Context, in *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
	if s.ple != nil {
		return s.ple(ctx, in)
	}
	return nil, nil
}

func (s *stubLogsService) CreateLogGroup(ctx context.Context, in *cloudwatchlogs.CreateLogGroupInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	if s.clg != nil {
		return s.clg(ctx, in)
	}
	return nil, nil
}

func (s *stubLogsService) CreateLogStream(ctx context.Context, in *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	if s.cls != nil {
		return s.cls(ctx, in)
	}
	return nil, nil
}

func (s *stubLogsService) PutRetentionPolicy(ctx context.Context, in *cloudwatchlogs.PutRetentionPolicyInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
	if s.prp != nil {
		return s.prp(ctx, in)
	}
	return nil, nil
}

func (s *stubLogsService) DescribeLogGroups(ctx context.Context, in *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	if s.dlg != nil {
		return s.dlg(ctx, in)
	}
	return nil, nil
}

type mockSender struct {
	mock.Mock
}

func (m *mockSender) Send(batch *logEventBatch) {
	m.Called(batch)
}

func (m *mockSender) SetRetryDuration(d time.Duration) {
	m.Called(d)
}

func (m *mockSender) RetryDuration() time.Duration {
	args := m.Called()
	return args.Get(0).(time.Duration)
}

func (m *mockSender) Stop() {
	m.Called()
}

func TestAddSingleEvent_WithAccountId(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	var called atomic.Bool
	expectedEntity := &types.Entity{
		Attributes: map[string]string{
			"PlatformType":         "AWS::EC2",
			"EC2.InstanceId":       "i-123456789",
			"EC2.AutoScalingGroup": "test-group",
		},
		KeyAttributes: map[string]string{
			"Name":         "myService",
			"Environment":  "myEnvironment",
			"AwsAccountId": "123456789",
		},
	}

	s.ple = func(_ context.Context, in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		called.Store(true)

		if *in.LogGroupName != "G" || *in.LogStreamName != "S" {
			t.Errorf("PutLogEvents called with wrong group and stream: %v/%v", *in.LogGroupName, *in.LogStreamName)
		}

		if len(in.LogEvents) != 1 || *in.LogEvents[0].Message != "MSG" {
			t.Errorf("PutLogEvents called with incorrect message, got: '%v'", *in.LogEvents[0].Message)
		}
		require.Equal(t, expectedEntity, in.Entity)
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	ep := newMockEntityProvider(expectedEntity)
	q, sender := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, ep, &wg)
	q.AddEvent(newStubLogEvent("MSG", time.Now()))
	require.False(t, called.Load(), "PutLogEvents has been called too fast, it should wait until FlushTimeout.")

	q.flushTimeout.Store(200 * time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	q.resetFlushTimer()

	time.Sleep(time.Second)
	require.True(t, called.Load(), "PutLogEvents has not been called after FlushTimeout has been reached.")

	q.Stop()
	sender.Stop()
	wg.Wait()
}

func TestAddSingleEvent_WithoutAccountId(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	var called atomic.Bool

	s.ple = func(_ context.Context, in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		called.Store(true)

		if *in.LogGroupName != "G" || *in.LogStreamName != "S" {
			t.Errorf("PutLogEvents called with wrong group and stream: %v/%v", *in.LogGroupName, *in.LogStreamName)
		}

		if len(in.LogEvents) != 1 || *in.LogEvents[0].Message != "MSG" {
			t.Errorf("PutLogEvents called with incorrect message, got: '%v'", *in.LogEvents[0].Message)
		}
		require.Nil(t, in.Entity)
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	ep := newMockEntityProvider(nil)
	q, sender := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, ep, &wg)
	q.AddEvent(newStubLogEvent("MSG", time.Now()))
	require.False(t, called.Load(), "PutLogEvents has been called too fast, it should wait until FlushTimeout.")

	q.flushTimeout.Store(time.Second)
	time.Sleep(10 * time.Millisecond)
	q.resetFlushTimer()

	time.Sleep(2 * time.Second)
	require.True(t, called.Load(), "PutLogEvents has not been called after FlushTimeout has been reached.")

	q.Stop()
	sender.Stop()
	wg.Wait()
}

func TestStopQueueWouldDoFinalSend(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	var called atomic.Bool

	s.ple = func(_ context.Context, in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		called.Store(true)
		if len(in.LogEvents) != 1 {
			t.Errorf("PutLogEvents called with incorrect number of message, expecting 1, but %v received", len(in.LogEvents))
		}
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	q, sender := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("MSG", time.Now()))

	time.Sleep(10 * time.Millisecond)

	require.False(t, called.Load(), "PutLogEvents has been called too fast, it should wait until FlushTimeout.")

	q.Stop()
	sender.Stop()
	wg.Wait()

	require.True(t, called.Load(), "PutLogEvents has not been called after FlushTimeout has been reached.")
}

func TestStopPusherWouldStopRetries(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService

	s.ple = func(context.Context, *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return nil, &types.ServiceUnavailableException{}
	}

	logSink := testutil.NewLogSink()
	q, sender := testPreparationWithLogger(t, logSink, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("MSG", time.Now()))
	time.Sleep(10 * time.Millisecond)

	triggerSend(t, q)
	// stop should try flushing the remaining events with retry disabled
	q.Stop()
	sender.Stop()

	time.Sleep(50 * time.Millisecond)
	wg.Wait()

	logLines := logSink.Lines()
	require.Equal(t, 3, len(logLines), fmt.Sprintf("Expecting 3 logs, but %d received", len(logLines)))
	lastLine := logLines[len(logLines)-1]
	require.True(t, strings.Contains(lastLine, "E!"))
	require.True(t, strings.Contains(lastLine, "Stop requested after 0 retries to G/S failed for PutLogEvents, request dropped"))
}

func TestLongMessageHandling(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService

	// This test was updated since truncation is now handled at the buffer reading level
	// We now verify that long messages are passed through without modification
	longMsg := strings.Repeat("x", 10000) // A long message that would have been truncated before

	s.ple = func(_ context.Context, in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if len(in.LogEvents) != 1 {
			t.Fatalf("PutLogEvents called with incorrect number of message, expecting 1, but %v received", len(in.LogEvents))
		}
		msg := *in.LogEvents[0].Message

		// Verify the message is passed through unchanged
		if msg != longMsg {
			t.Errorf("Long message was modified: expected length %d, got %d", len(longMsg), len(msg))
		}

		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	q, sender := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent(longMsg, time.Now()))

	triggerSend(t, q)
	q.Stop()
	sender.Stop()
	wg.Wait()
}

func TestRequestIsLessThan1MB(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	// Use a large message but less than the AWS CloudWatch Logs limit
	longMsg := strings.Repeat("x", 200000) // 200KB

	s.ple = func(_ context.Context, in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		length := 0
		for _, le := range in.LogEvents {
			length += len(*le.Message) + perEventHeaderBytes
		}

		if length > reqSizeLimit {
			t.Fatalf("PutLogEvents called with payload larger than request limit of 1MB, %v received", length)
		}

		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	q, sender := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	for i := 0; i < 8; i++ {
		q.AddEvent(newStubLogEvent(longMsg, time.Now()))
	}
	time.Sleep(10 * time.Millisecond)
	triggerSend(t, q)
	triggerSend(t, q)
	q.Stop()
	sender.Stop()
	wg.Wait()
}

func TestRequestIsLessThan10kEvents(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	msg := "m"

	s.ple = func(_ context.Context, in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if len(in.LogEvents) > 10000 {
			t.Fatalf("PutLogEvents called with more than 10k events, %v received", len(in.LogEvents))
		}

		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	q, sender := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	for i := 0; i < 30000; i++ {
		q.AddEvent(newStubLogEvent(msg, time.Now()))
	}
	time.Sleep(10 * time.Millisecond)
	for i := 0; i < 5; i++ {
		triggerSend(t, q)
	}
	q.Stop()
	sender.Stop()
	wg.Wait()
}

func TestTimestampPopulation(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService

	s.ple = func(_ context.Context, in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if len(in.LogEvents) > 10000 {
			t.Fatalf("PutLogEvents called with more than 10k events, %v received", len(in.LogEvents))
		}

		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	q, sender := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	for i := 0; i < 3; i++ {
		q.AddEvent(newStubLogEvent("msg", time.Time{}))
	}
	time.Sleep(10 * time.Millisecond)
	for i := 0; i < 5; i++ {
		triggerSend(t, q)
	}
	q.Stop()
	sender.Stop()
	wg.Wait()
}

func TestIgnoreOutOfTimeRangeEvent(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService

	s.ple = func(context.Context, *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		t.Errorf("PutLogEvents should not be called for out of range events")
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	logSink := testutil.NewLogSink()
	q, sender := testPreparationWithLogger(t, logSink, -1, &s, 10*time.Millisecond, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("MSG", time.Now().Add(-15*24*time.Hour)))
	q.AddEventNonBlocking(newStubLogEvent("MSG", time.Now().Add(2*time.Hour+1*time.Minute)))

	logLines := logSink.Lines()
	require.Equal(t, 2, len(logLines), fmt.Sprintf("Expecting 2 error logs, but %d received", len(logLines)))

	for _, logLine := range logLines {
		require.True(t, strings.Contains(logLine, "E!"), fmt.Sprintf("Expecting error log with unhandled error, but received '%s' in the log", logSink))
		require.True(t, strings.Contains(logLine, "Discard the log entry"), fmt.Sprintf("Expecting error log with unhandled error, but received '%s' in the log", logSink))
	}

	time.Sleep(20 * time.Millisecond)
	q.Stop()
	sender.Stop()
	wg.Wait()
}

func TestAddMultipleEvents(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService

	s.ple = func(_ context.Context, in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if *in.LogGroupName != "G" || *in.LogStreamName != "S" {
			t.Errorf("PutLogEvents called with wrong group and stream: %v/%v", *in.LogGroupName, *in.LogStreamName)
		}

		if len(in.LogEvents) != 100 {
			t.Errorf("PutLogEvents called with incorrect number of message, only %v received", len(in.LogEvents))
		}

		for i, le := range in.LogEvents {
			if *le.Message != fmt.Sprintf("MSG - %v", i) {
				t.Errorf("PutLogEvents received message in wrong order expect 'MSG - %d', but got %v", i, *le.Message)
			}
			if i != 0 && *le.Timestamp < *in.LogEvents[i-1].Timestamp {
				t.Errorf("PutLogEvents received message in wrong order")
			}
		}

		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	var evts []logs.LogEvent
	start := time.Now().Add(-100 * time.Millisecond)
	for i := 0; i < 100; i++ {
		evts = append(evts, newStubLogEvent(
			fmt.Sprintf("MSG - %v", i),
			start.Add(time.Duration(i)*time.Millisecond),
		))
	}
	evts[10], evts[90] = evts[90], evts[10] // make events out of order
	q, sender := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	for _, e := range evts {
		q.AddEvent(e)
	}

	q.flushTimeout.Store(10 * time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	q.resetFlushTimer()

	time.Sleep(time.Second)

	q.Stop()
	sender.Stop()
	wg.Wait()
}

func TestSendReqWhenEventsSpanMoreThan24Hrs(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	var ci atomic.Int32

	s.ple = func(_ context.Context, in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if ci.Load() == 0 {
			if len(in.LogEvents) != 3 {
				t.Errorf("PutLogEvents called with incorrect number of message, expecting 3, but %v received", len(in.LogEvents))
			}

			for _, le := range in.LogEvents {
				if *le.Message == "MSG now" {
					t.Errorf("PutLogEvents received wrong message, '%v' should not be sent together with the previous messages", *le.Message)
				}
			}

			ci.Add(1)
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		} else if ci.Load() == 1 {
			if len(in.LogEvents) != 1 {
				t.Errorf("PutLogEvents called with incorrect number of message, expecting 1, but %v received", len(in.LogEvents))
			}

			le := in.LogEvents[0]
			if *le.Message != "MSG now" {
				t.Errorf("PutLogEvents received wrong message: '%v'", *le.Message)
			}
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		}

		t.Errorf("PutLogEvents should not be call more the 2 times")
		return nil, nil
	}

	q, sender := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("MSG 25hrs ago", time.Now().Add(-25*time.Hour)))
	q.AddEvent(newStubLogEvent("MSG 24hrs ago", time.Now().Add(-24*time.Hour)))
	q.AddEvent(newStubLogEvent("MSG 23hrs ago", time.Now().Add(-23*time.Hour)))
	q.AddEvent(newStubLogEvent("MSG now", time.Now()))
	q.flushTimeout.Store(10 * time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	q.resetFlushTimer()
	time.Sleep(20 * time.Millisecond)
	q.Stop()
	sender.Stop()
	wg.Wait()
}

func TestUnhandledErrorWouldNotResend(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	var cnt atomic.Int32

	s.ple = func(context.Context, *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if cnt.Load() == 0 {
			cnt.Add(1)
			return nil, errors.New("unhandled error")
		}
		t.Errorf("Pusher should not attempt a resend when an unhandled error has been returned")
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	logSink := testutil.NewLogSink()
	q, sender := testPreparationWithLogger(t, logSink, -1, &s, 10*time.Millisecond, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("msg", time.Now()))
	time.Sleep(2 * time.Second)

	logLine := logSink.String()
	require.True(t, strings.Contains(logLine, "E!"), fmt.Sprintf("Expecting error log with unhandled error, but received '%s' in the log", logLine))
	require.True(t, strings.Contains(logLine, "unhandled error"), fmt.Sprintf("Expecting error log with unhandled error, but received '%s' in the log", logLine))

	q.Stop()
	sender.Stop()
	wg.Wait()
	require.EqualValues(t, 1, cnt.Load(), fmt.Sprintf("Expecting pusher to call send 1 time, but %d times called", cnt.Load()))
}

func TestCreateLogGroupAndLogStreamWhenNotFound(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService

	var plec, clgc, clsc atomic.Int32
	s.ple = func(context.Context, *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		var e error
		switch plec.Load() {
		case 0:
			e = &types.ResourceNotFoundException{}
		case 1:
			e = &smithy.GenericAPIError{Code: "Unknown Error", Message: ""}
		case 2:
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		default:
			t.Errorf("Unexpected PutLogEvents call (%d time)", plec.Load())
		}
		plec.Add(1)
		return nil, e
	}

	s.clg = func(context.Context, *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
		clgc.Add(1)
		return nil, nil
	}
	s.cls = func(context.Context, *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
		clsc.Add(1)
		return nil, nil
	}

	logSink := testutil.NewLogSink()
	q, sender := testPreparationWithLogger(t, logSink, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	var eventWG sync.WaitGroup
	eventWG.Add(1)
	q.AddEvent(&stubLogEvent{message: "msg", timestamp: time.Now(), done: eventWG.Done})
	time.Sleep(10 * time.Millisecond)
	triggerSend(t, q)

	eventWG.Wait()
	foundUnknownErr := false
	logLines := logSink.Lines()
	for _, logLine := range logLines {
		if strings.Contains(logLine, "E!") && strings.Contains(logLine, "Unknown Error") {
			foundUnknownErr = true
		}
	}

	require.True(t, foundUnknownErr, fmt.Sprintf("Expecting error log with unknown error, but received '%s' in the log", logSink))

	q.Stop()
	sender.Stop()
	wg.Wait()
}

func TestLogRejectedLogEntryInfo(t *testing.T) {
	var wg sync.WaitGroup
	var s stubLogsService

	s.ple = func(context.Context, *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return &cloudwatchlogs.PutLogEventsOutput{
			RejectedLogEventsInfo: &types.RejectedLogEventsInfo{
				TooOldLogEventEndIndex:   aws.Int32(100),
				TooNewLogEventStartIndex: aws.Int32(200),
				ExpiredLogEventEndIndex:  aws.Int32(300),
			},
		}, nil
	}

	logSink := testutil.NewLogSink()
	q, sender := testPreparationWithLogger(t, logSink, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	var eventWG sync.WaitGroup
	eventWG.Add(1)
	q.AddEvent(&stubLogEvent{message: "msg", timestamp: time.Now(), done: eventWG.Done})
	time.Sleep(10 * time.Millisecond)
	triggerSend(t, q)

	eventWG.Wait()
	logLines := logSink.Lines()
	require.Len(t, logLines, 4, fmt.Sprintf("Expecting 3 error logs, but %d received", len(logLines)))

	logLine := logLines[0]
	require.True(t, strings.Contains(logLine, "W!"), fmt.Sprintf("Expecting error log events too old, but received '%s' in the log", logSink.String()))
	require.True(t, strings.Contains(logLine, "100"), fmt.Sprintf("Expecting error log events too old, but received '%s' in the log", logSink.String()))

	logLine = logLines[1]
	require.True(t, strings.Contains(logLine, "W!"), fmt.Sprintf("Expecting error log events too new, but received '%s' in the log", logSink.String()))
	require.True(t, strings.Contains(logLine, "200"), fmt.Sprintf("Expecting error log events too new, but received '%s' in the log", logSink.String()))

	logLine = logLines[2]
	require.True(t, strings.Contains(logLine, "W!"), fmt.Sprintf("Expecting error log events too expired, but received '%s' in the log", logSink.String()))
	require.True(t, strings.Contains(logLine, "300"), fmt.Sprintf("Expecting error log events too expired, but received '%s' in the log", logSink.String()))

	q.Stop()
	sender.Stop()
	wg.Wait()
}

func TestAddEventNonBlocking(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	const N = 100

	s.ple = func(_ context.Context, in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if len(in.LogEvents) != N {
			t.Errorf("PutLogEvents called with incorrect number of message, only %v received", len(in.LogEvents))
		}

		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	var evts []logs.LogEvent
	start := time.Now().Add(-N * time.Millisecond)
	for i := 0; i < N; i++ {
		evts = append(evts, newStubLogEvent(
			fmt.Sprintf("MSG - %v", i),
			start.Add(time.Duration(i)*time.Millisecond),
		))
	}
	q, sender := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	time.Sleep(200 * time.Millisecond) // Wait until pusher started, merge channel is blocked

	for _, e := range evts {
		q.AddEventNonBlocking(e)
	}

	time.Sleep(time.Second)
	triggerSend(t, q)
	time.Sleep(20 * time.Millisecond)

	q.Stop()
	sender.Stop()
	wg.Wait()
}

func TestResendWouldStopAfterExhaustedRetries(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	var cnt atomic.Int32

	s.ple = func(context.Context, *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		cnt.Add(1)
		return nil, &types.ServiceUnavailableException{}
	}

	logSink := testutil.NewLogSink()
	q, sender := testPreparationWithLogger(t, logSink, -1, &s, 10*time.Millisecond, time.Second, nil, &wg)
	q.AddEvent(newStubLogEvent("msg", time.Now()))
	time.Sleep(2 * time.Second)

	logLines := logSink.Lines()
	lastLine := logLines[len(logLines)-1]
	expected := fmt.Sprintf("All %v retries to G/S failed for PutLogEvents, request dropped.", cnt.Load()-1)
	require.True(t, strings.HasSuffix(lastLine, expected), fmt.Sprintf("Expecting error log to end with request dropped, but received '%s' in the log", logSink.String()))

	q.Stop()
	sender.Stop()
	wg.Wait()
}

// Cannot call q.send() directly as it would cause a race condition. Reset last sent time and trigger flush.
func triggerSend(t *testing.T, q *queue) {
	t.Helper()
	q.lastSentTime.Store(time.Time{})
	q.flushCh <- struct{}{}
}

func testPreparation(
	t *testing.T,
	retention int32,
	service cloudWatchLogsService,
	flushTimeout time.Duration,
	retryDuration time.Duration,
	entityProvider logs.LogEntityProvider,
	wg *sync.WaitGroup,
) (*queue, Sender) {
	return testPreparationWithLogger(
		t,
		testutil.NewNopLogger(),
		retention,
		service,
		flushTimeout,
		retryDuration,
		entityProvider,
		wg,
	)
}

func testPreparationWithLogger(
	t *testing.T,
	logger telegraf.Logger,
	retention int32,
	service cloudWatchLogsService,
	flushTimeout time.Duration,
	retryDuration time.Duration,
	entityProvider logs.LogEntityProvider,
	wg *sync.WaitGroup,
) (*queue, Sender) {
	t.Helper()
	tm := NewTargetManager(logger, service)
	s := newSender(logger, service, tm, retryDuration)
	q := newQueue(
		logger,
		Target{"G", "S", util.StandardLogGroupClass, retention},
		flushTimeout,
		entityProvider,
		s,
		wg,
	)
	return q.(*queue), s
}

func TestQueueCallbackRegistration(t *testing.T) {
	t.Run("RegistersCallbacks", func(t *testing.T) {
		var wg sync.WaitGroup
		var s stubLogsService
		var called bool

		// Mock the PutLogEvents method to verify the batch has callbacks registered
		s.ple = func(context.Context, *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
			called = true
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		}

		mockSender := &mockSender{}
		mockSender.On("Send", mock.AnythingOfType("*pusher.logEventBatch")).Run(func(args mock.Arguments) {
			batch := args.Get(0).(*logEventBatch)

			assert.NotEmpty(t, batch.doneCallbacks, "Regular callbacks should be registered")
			assert.Empty(t, batch.stateCallbacks, "State callbacks should not be registered")

			_, err := s.PutLogEvents(t.Context(), batch.build())
			assert.NoError(t, err)
		}).Return()

		logger := testutil.NewNopLogger()
		q := &queue{
			target:          Target{"G", "S", util.StandardLogGroupClass, -1},
			logger:          logger,
			converter:       newConverter(logger, Target{"G", "S", util.StandardLogGroupClass, -1}),
			batch:           newLogEventBatch(Target{"G", "S", util.StandardLogGroupClass, -1}, nil),
			sender:          mockSender,
			eventsCh:        make(chan logs.LogEvent, 100),
			flushCh:         make(chan struct{}),
			resetTimerCh:    make(chan struct{}),
			flushTimer:      time.NewTimer(10 * time.Millisecond),
			startNonBlockCh: make(chan struct{}),
			wg:              &wg,
		}
		q.flushTimeout.Store(10 * time.Millisecond)

		q.batch.append(newLogEvent(time.Now(), "test message", nil))
		q.send()

		mockSender.AssertExpectations(t)
		assert.True(t, called, "PutLogEvents should have been called")
	})

	t.Run("RegistersStateCallbacksForStatefulEvents", func(t *testing.T) {
		var wg sync.WaitGroup

		mrq := &mockRangeQueue{}
		mrq.On("ID").Return("test-queue")
		mrq.On("Enqueue", mock.Anything).Return()

		mockSender := &mockSender{}
		mockSender.On("Send", mock.AnythingOfType("*pusher.logEventBatch")).Run(func(args mock.Arguments) {
			batch := args.Get(0).(*logEventBatch)

			assert.NotEmpty(t, batch.doneCallbacks, "Regular callbacks should be registered")
			assert.NotEmpty(t, batch.stateCallbacks, "State callbacks should be registered")

			batcher, ok := batch.batchers["test-queue"]
			assert.True(t, ok, "Batch should have a batcher for our queue")
			assert.NotNil(t, batcher, "Batcher should not be nil")
		}).Return()

		logger := testutil.NewNopLogger()
		q := &queue{
			target:          Target{"G", "S", util.StandardLogGroupClass, -1},
			logger:          logger,
			converter:       newConverter(logger, Target{"G", "S", util.StandardLogGroupClass, -1}),
			batch:           newLogEventBatch(Target{"G", "S", util.StandardLogGroupClass, -1}, nil),
			sender:          mockSender,
			eventsCh:        make(chan logs.LogEvent, 100),
			flushCh:         make(chan struct{}),
			resetTimerCh:    make(chan struct{}),
			flushTimer:      time.NewTimer(10 * time.Millisecond),
			startNonBlockCh: make(chan struct{}),
			wg:              &wg,
		}
		q.flushTimeout.Store(10 * time.Millisecond)

		event := newStubStatefulLogEvent("test message", time.Now(), state.NewRange(10, 20), mrq)

		convertedEvent := q.converter.convert(event)
		q.batch.append(convertedEvent)

		q.send()

		mockSender.AssertExpectations(t)
	})
}
