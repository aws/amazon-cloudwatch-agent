// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/influxdata/telegraf"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

type stubLogsService struct {
	ple func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error)
	clg func(input *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error)
	cls func(input *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error)
	prp func(input *cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error)
	dlg func(input *cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
}

func (s *stubLogsService) PutLogEvents(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
	if s.ple != nil {
		return s.ple(in)
	}
	return nil, nil
}

func (s *stubLogsService) CreateLogGroup(in *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	if s.clg != nil {
		return s.clg(in)
	}
	return nil, nil
}

func (s *stubLogsService) CreateLogStream(in *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	if s.cls != nil {
		return s.cls(in)
	}
	return nil, nil
}

func (s *stubLogsService) PutRetentionPolicy(in *cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
	if s.prp != nil {
		return s.prp(in)
	}
	return nil, nil
}

func (s *stubLogsService) DescribeLogGroups(in *cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	if s.dlg != nil {
		return s.dlg(in)
	}
	return nil, nil
}

func TestAddSingleEvent_WithAccountId(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	var called atomic.Bool
	expectedEntity := &cloudwatchlogs.Entity{
		Attributes: map[string]*string{
			"PlatformType":         aws.String("AWS::EC2"),
			"EC2.InstanceId":       aws.String("i-123456789"),
			"EC2.AutoScalingGroup": aws.String("test-group"),
		},
		KeyAttributes: map[string]*string{
			"Name":         aws.String("myService"),
			"Environment":  aws.String("myEnvironment"),
			"AwsAccountId": aws.String("123456789"),
		},
	}

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
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
	stop, q := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, ep, &wg)
	q.AddEvent(newStubLogEvent("MSG", time.Now()))
	require.False(t, called.Load(), "PutLogEvents has been called too fast, it should wait until FlushTimeout.")

	q.flushTimeout.Store(200 * time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	q.resetFlushTimer()

	time.Sleep(time.Second)
	require.True(t, called.Load(), "PutLogEvents has not been called after FlushTimeout has been reached.")

	close(stop)
	wg.Wait()
}

func TestAddSingleEvent_WithoutAccountId(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	var called atomic.Bool

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
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
	stop, q := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, ep, &wg)
	q.AddEvent(newStubLogEvent("MSG", time.Now()))
	require.False(t, called.Load(), "PutLogEvents has been called too fast, it should wait until FlushTimeout.")

	q.flushTimeout.Store(time.Second)
	time.Sleep(10 * time.Millisecond)
	q.resetFlushTimer()

	time.Sleep(2 * time.Second)
	require.True(t, called.Load(), "PutLogEvents has not been called after FlushTimeout has been reached.")

	close(stop)
	wg.Wait()
}

func TestStopQueueWouldDoFinalSend(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	var called atomic.Bool

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		called.Store(true)
		if len(in.LogEvents) != 1 {
			t.Errorf("PutLogEvents called with incorrect number of message, expecting 1, but %v received", len(in.LogEvents))
		}
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	stop, q := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("MSG", time.Now()))

	time.Sleep(10 * time.Millisecond)

	require.False(t, called.Load(), "PutLogEvents has been called too fast, it should wait until FlushTimeout.")

	close(stop)
	wg.Wait()

	require.True(t, called.Load(), "PutLogEvents has not been called after FlushTimeout has been reached.")
}

func TestStopPusherWouldStopRetries(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService

	s.ple = func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return nil, &cloudwatchlogs.ServiceUnavailableException{}
	}

	logSink := testutil.NewLogSink()
	stop, q := testPreparationWithLogger(t, logSink, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("MSG", time.Now()))
	time.Sleep(10 * time.Millisecond)

	triggerSend(t, q)
	// stop should try flushing the remaining events with retry disabled
	close(stop)

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

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
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

	stop, q := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent(longMsg, time.Now()))

	triggerSend(t, q)
	close(stop)
	wg.Wait()
}

func TestRequestIsLessThan1MB(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	// Use a large message but less than the AWS CloudWatch Logs limit
	longMsg := strings.Repeat("x", 200000) // 200KB

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		length := 0
		for _, le := range in.LogEvents {
			length += len(*le.Message) + perEventHeaderBytes
		}

		if length > reqSizeLimit {
			t.Fatalf("PutLogEvents called with payload larger than request limit of 1MB, %v received", length)
		}

		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	stop, q := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	for i := 0; i < 8; i++ {
		q.AddEvent(newStubLogEvent(longMsg, time.Now()))
	}
	time.Sleep(10 * time.Millisecond)
	triggerSend(t, q)
	triggerSend(t, q)
	close(stop)
	wg.Wait()
}

func TestRequestIsLessThan10kEvents(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	msg := "m"

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if len(in.LogEvents) > 10000 {
			t.Fatalf("PutLogEvents called with more than 10k events, %v received", len(in.LogEvents))
		}

		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	stop, q := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	for i := 0; i < 30000; i++ {
		q.AddEvent(newStubLogEvent(msg, time.Now()))
	}
	time.Sleep(10 * time.Millisecond)
	for i := 0; i < 5; i++ {
		triggerSend(t, q)
	}
	close(stop)
	wg.Wait()
}

func TestTimestampPopulation(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if len(in.LogEvents) > 10000 {
			t.Fatalf("PutLogEvents called with more than 10k events, %v received", len(in.LogEvents))
		}

		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	stop, q := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	for i := 0; i < 3; i++ {
		q.AddEvent(newStubLogEvent("msg", time.Time{}))
	}
	time.Sleep(10 * time.Millisecond)
	for i := 0; i < 5; i++ {
		triggerSend(t, q)
	}
	close(stop)
	wg.Wait()
}

func TestIgnoreOutOfTimeRangeEvent(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService

	s.ple = func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		t.Errorf("PutLogEvents should not be called for out of range events")
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	logSink := testutil.NewLogSink()
	stop, q := testPreparationWithLogger(t, logSink, -1, &s, 10*time.Millisecond, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("MSG", time.Now().Add(-15*24*time.Hour)))
	q.AddEventNonBlocking(newStubLogEvent("MSG", time.Now().Add(2*time.Hour+1*time.Minute)))

	logLines := logSink.Lines()
	require.Equal(t, 2, len(logLines), fmt.Sprintf("Expecting 2 error logs, but %d received", len(logLines)))

	for _, logLine := range logLines {
		require.True(t, strings.Contains(logLine, "E!"), fmt.Sprintf("Expecting error log with unhandled error, but received '%s' in the log", logSink))
		require.True(t, strings.Contains(logLine, "Discard the log entry"), fmt.Sprintf("Expecting error log with unhandled error, but received '%s' in the log", logSink))
	}

	time.Sleep(20 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestAddMultipleEvents(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
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
	stop, q := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	for _, e := range evts {
		q.AddEvent(e)
	}

	q.flushTimeout.Store(10 * time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	q.resetFlushTimer()

	time.Sleep(time.Second)

	close(stop)
	wg.Wait()
}

func TestSendReqWhenEventsSpanMoreThan24Hrs(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	var ci atomic.Int32

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
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

	stop, q := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("MSG 25hrs ago", time.Now().Add(-25*time.Hour)))
	q.AddEvent(newStubLogEvent("MSG 24hrs ago", time.Now().Add(-24*time.Hour)))
	q.AddEvent(newStubLogEvent("MSG 23hrs ago", time.Now().Add(-23*time.Hour)))
	q.AddEvent(newStubLogEvent("MSG now", time.Now()))
	q.flushTimeout.Store(10 * time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	q.resetFlushTimer()
	time.Sleep(20 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestUnhandledErrorWouldNotResend(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	var cnt atomic.Int32

	s.ple = func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if cnt.Load() == 0 {
			cnt.Add(1)
			return nil, errors.New("unhandled error")
		}
		t.Errorf("Pusher should not attempt a resend when an unhandled error has been returned")
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	logSink := testutil.NewLogSink()
	stop, q := testPreparationWithLogger(t, logSink, -1, &s, 10*time.Millisecond, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("msg", time.Now()))
	time.Sleep(2 * time.Second)

	logLine := logSink.String()
	require.True(t, strings.Contains(logLine, "E!"), fmt.Sprintf("Expecting error log with unhandled error, but received '%s' in the log", logLine))
	require.True(t, strings.Contains(logLine, "unhandled error"), fmt.Sprintf("Expecting error log with unhandled error, but received '%s' in the log", logLine))

	close(stop)
	wg.Wait()
	require.EqualValues(t, 1, cnt.Load(), fmt.Sprintf("Expecting pusher to call send 1 time, but %d times called", cnt.Load()))
}

func TestCreateLogGroupAndLogStreamWhenNotFound(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService

	var plec, clgc, clsc atomic.Int32
	s.ple = func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		var e error
		switch plec.Load() {
		case 0:
			e = &cloudwatchlogs.ResourceNotFoundException{}
		case 1:
			e = awserr.New("Unknown Error", "", nil)
		case 2:
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		default:
			t.Errorf("Unexpected PutLogEvents call (%d time)", plec.Load())
		}
		plec.Add(1)
		return nil, e
	}

	s.clg = func(*cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
		clgc.Add(1)
		return nil, nil
	}
	s.cls = func(*cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
		clsc.Add(1)
		return nil, nil
	}

	logSink := testutil.NewLogSink()
	stop, q := testPreparationWithLogger(t, logSink, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
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

	close(stop)
	wg.Wait()
}

func TestLogRejectedLogEntryInfo(t *testing.T) {
	var wg sync.WaitGroup
	var s stubLogsService

	s.ple = func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return &cloudwatchlogs.PutLogEventsOutput{
			RejectedLogEventsInfo: &cloudwatchlogs.RejectedLogEventsInfo{
				TooOldLogEventEndIndex:   aws.Int64(100),
				TooNewLogEventStartIndex: aws.Int64(200),
				ExpiredLogEventEndIndex:  aws.Int64(300),
			},
		}, nil
	}

	logSink := testutil.NewLogSink()
	stop, q := testPreparationWithLogger(t, logSink, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
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

	close(stop)
	wg.Wait()
}

func TestAddEventNonBlocking(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	const N = 100

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
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
	stop, q := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	time.Sleep(200 * time.Millisecond) // Wait until pusher started, merge channel is blocked

	for _, e := range evts {
		q.AddEventNonBlocking(e)
	}

	time.Sleep(time.Second)
	triggerSend(t, q)
	time.Sleep(20 * time.Millisecond)

	close(stop)
	wg.Wait()
}

func TestResendWouldStopAfterExhaustedRetries(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	var cnt atomic.Int32

	s.ple = func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		cnt.Add(1)
		return nil, &cloudwatchlogs.ServiceUnavailableException{}
	}

	logSink := testutil.NewLogSink()
	stop, q := testPreparationWithLogger(t, logSink, -1, &s, 10*time.Millisecond, time.Second, nil, &wg)
	q.AddEvent(newStubLogEvent("msg", time.Now()))
	time.Sleep(2 * time.Second)

	logLines := logSink.Lines()
	lastLine := logLines[len(logLines)-1]
	expected := fmt.Sprintf("All %v retries to G/S failed for PutLogEvents, request dropped.", cnt.Load()-1)
	require.True(t, strings.HasSuffix(lastLine, expected), fmt.Sprintf("Expecting error log to end with request dropped, but received '%s' in the log", logSink.String()))

	close(stop)
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
	retention int,
	service cloudWatchLogsService,
	flushTimeout time.Duration,
	retryDuration time.Duration,
	entityProvider logs.LogEntityProvider,
	wg *sync.WaitGroup,
) (chan struct{}, *queue) {
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
	retention int,
	service cloudWatchLogsService,
	flushTimeout time.Duration,
	retryDuration time.Duration,
	entityProvider logs.LogEntityProvider,
	wg *sync.WaitGroup,
) (chan struct{}, *queue) {
	t.Helper()
	stop := make(chan struct{})
	tm := NewTargetManager(logger, service)
	s := newSender(logger, service, tm, retryDuration, stop)
	q := newQueue(
		logger,
		Target{"G", "S", util.StandardLogGroupClass, retention},
		flushTimeout,
		entityProvider,
		s,
		stop,
		wg,
	)
	return stop, q.(*queue)
}
