// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
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
	called := false
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
		called = true

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
	stop, q := testPreparation(-1, &s, 1*time.Hour, 2*time.Hour, ep, &wg)
	q.AddEvent(newStubLogEvent("MSG", time.Now()))
	require.False(t, called, "PutLogEvents has been called too fast, it should wait until FlushTimeout.")

	q.flushTimeout = time.Second
	q.resetFlushTimer()

	time.Sleep(2 * time.Second)
	require.True(t, called, "PutLogEvents has not been called after FlushTimeout has been reached.")

	close(stop)
	wg.Wait()
}

func TestAddSingleEvent_WithoutAccountId(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	called := false

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		called = true

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
	stop, q := testPreparation(-1, &s, 1*time.Hour, 2*time.Hour, ep, &wg)
	q.AddEvent(newStubLogEvent("MSG", time.Now()))
	require.False(t, called, "PutLogEvents has been called too fast, it should wait until FlushTimeout.")

	q.flushTimeout = time.Second
	q.resetFlushTimer()

	time.Sleep(2 * time.Second)
	require.True(t, called, "PutLogEvents has not been called after FlushTimeout has been reached.")

	close(stop)
	wg.Wait()
}

func TestStopQueueWouldDoFinalSend(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	called := false

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		called = true
		if len(in.LogEvents) != 1 {
			t.Errorf("PutLogEvents called with incorrect number of message, expecting 1, but %v received", len(in.LogEvents))
		}
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	stop, q := testPreparation(-1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("MSG", time.Now()))

	time.Sleep(10 * time.Millisecond)

	require.False(t, called, "PutLogEvents has been called too fast, it should wait until FlushTimeout.")

	close(stop)
	wg.Wait()

	require.True(t, called, "PutLogEvents has not been called after FlushTimeout has been reached.")
}

func TestStopPusherWouldStopRetries(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return nil, &cloudwatchlogs.ServiceUnavailableException{}
	}

	stop, q := testPreparation(-1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("MSG", time.Now()))

	sendComplete := make(chan struct{})

	go func() {
		defer close(sendComplete)
		q.send()
	}()

	close(stop)

	select {
	case <-time.After(50 * time.Millisecond):
		t.Errorf("send did not quit retrying after p has been Stopped.")
	case <-sendComplete:
	}
}

func TestLongMessageGetsTruncated(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	longMsg := strings.Repeat("x", msgSizeLimit+1)

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if len(in.LogEvents) != 1 {
			t.Fatalf("PutLogEvents called with incorrect number of message, expecting 1, but %v received", len(in.LogEvents))
		}
		msg := *in.LogEvents[0].Message
		if msg == longMsg {
			t.Errorf("Long message was not truncated correctly")
		}

		if len(msg) > msgSizeLimit {
			t.Errorf("Truncated long message is still too long: %v observed, max allowed length is %v", len(msg), msgSizeLimit)
		}

		if !strings.HasSuffix(msg, truncatedSuffix) {
			t.Errorf("Truncated long message had the wrong suffix: %v", msg[len(msg)-30:])
		}

		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	stop, q := testPreparation(-1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent(longMsg, time.Now()))

	for len(q.batch.events) < 1 {
		time.Sleep(10 * time.Millisecond)
	}

	q.send()
	close(stop)
	wg.Wait()
}

func TestRequestIsLessThan1MB(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	longMsg := strings.Repeat("x", msgSizeLimit)

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

	stop, q := testPreparation(-1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	for i := 0; i < 8; i++ {
		q.AddEvent(newStubLogEvent(longMsg, time.Now()))
	}
	time.Sleep(10 * time.Millisecond)
	q.send()
	q.send()
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

	stop, q := testPreparation(-1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	for i := 0; i < 30000; i++ {
		q.AddEvent(newStubLogEvent(msg, time.Now()))
	}
	time.Sleep(10 * time.Millisecond)
	for i := 0; i < 5; i++ {
		q.send()
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

	stop, q := testPreparation(-1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	for i := 0; i < 3; i++ {
		q.AddEvent(newStubLogEvent("msg", time.Time{}))
	}
	time.Sleep(10 * time.Millisecond)
	for i := 0; i < 5; i++ {
		q.send()
	}
	close(stop)
	wg.Wait()
}

func TestIgnoreOutOfTimeRangeEvent(t *testing.T) {
	var wg sync.WaitGroup
	var s stubLogsService

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		t.Errorf("PutLogEvents should not be called for out of range events")
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	var logbuf bytes.Buffer
	log.SetOutput(io.MultiWriter(&logbuf, os.Stdout))

	stop, q := testPreparation(-1, &s, 10*time.Millisecond, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("MSG", time.Now().Add(-15*24*time.Hour)))
	q.AddEventNonBlocking(newStubLogEvent("MSG", time.Now().Add(2*time.Hour+1*time.Minute)))

	loglines := strings.Split(strings.TrimSpace(logbuf.String()), "\n")
	require.Equal(t, 2, len(loglines), fmt.Sprintf("Expecting 2 error logs, but %d received", len(loglines)))

	for _, logline := range loglines {
		require.True(t, strings.Contains(logline, "E!"), fmt.Sprintf("Expecting error log with unhandled error, but received '%s' in the log", logbuf.String()))
		require.True(t, strings.Contains(logline, "Discard the log entry"), fmt.Sprintf("Expecting error log with unhandled error, but received '%s' in the log", logbuf.String()))
	}

	log.SetOutput(os.Stderr)

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
	stop, q := testPreparation(-1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	for _, e := range evts {
		q.AddEvent(e)
	}

	q.flushTimeout = 10 * time.Millisecond
	q.resetFlushTimer()

	time.Sleep(time.Second)

	close(stop)
	wg.Wait()
}

func TestSendReqWhenEventsSpanMoreThan24Hrs(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService

	ci := 0
	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if ci == 0 {
			if len(in.LogEvents) != 3 {
				t.Errorf("PutLogEvents called with incorrect number of message, expecting 3, but %v received", len(in.LogEvents))
			}

			for _, le := range in.LogEvents {
				if *le.Message == "MSG now" {
					t.Errorf("PutLogEvents received wrong message, '%v' should not be sent together with the previous messages", *le.Message)
				}
			}

			ci++
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		} else if ci == 1 {
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

	stop, q := testPreparation(-1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("MSG 25hrs ago", time.Now().Add(-25*time.Hour)))
	q.AddEvent(newStubLogEvent("MSG 24hrs ago", time.Now().Add(-24*time.Hour)))
	q.AddEvent(newStubLogEvent("MSG 23hrs ago", time.Now().Add(-23*time.Hour)))
	q.AddEvent(newStubLogEvent("MSG now", time.Now()))
	q.flushTimeout = 10 * time.Millisecond
	q.resetFlushTimer()
	time.Sleep(20 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestUnhandledErrorWouldNotResend(t *testing.T) {
	var wg sync.WaitGroup
	var s stubLogsService

	cnt := 0
	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if cnt == 0 {
			cnt++
			return nil, errors.New("unhandled error")
		}
		t.Errorf("Pusher should not attempt a resend when an unhandled error has been returned")
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	var logbuf bytes.Buffer
	log.SetOutput(io.MultiWriter(&logbuf, os.Stdout))

	stop, q := testPreparation(-1, &s, 10*time.Millisecond, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("msg", time.Now()))
	time.Sleep(2 * time.Second)

	logline := logbuf.String()
	require.True(t, strings.Contains(logline, "E!"), fmt.Sprintf("Expecting error log with unhandled error, but received '%s' in the log", logbuf.String()))
	require.True(t, strings.Contains(logline, "unhandled error"), fmt.Sprintf("Expecting error log with unhandled error, but received '%s' in the log", logbuf.String()))

	log.SetOutput(os.Stderr)

	close(stop)
	wg.Wait()
	require.Equal(t, 1, cnt, fmt.Sprintf("Expecting pusher to call send 1 time, but %d times called", cnt))
}

func TestCreateLogGroupAndLogStreamWhenNotFound(t *testing.T) {
	var wg sync.WaitGroup
	var s stubLogsService

	var plec, clgc, clsc int
	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		var e error
		switch plec {
		case 0:
			e = &cloudwatchlogs.ResourceNotFoundException{}
		case 1:
			e = awserr.New("Unknown Error", "", nil)
		case 2:
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		default:
			t.Errorf("Unexpected PutLogEvents call (%d time)", plec)
		}
		plec++
		return nil, e
	}

	s.clg = func(in *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
		clgc++
		return nil, nil
	}
	s.cls = func(in *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
		clsc++
		return nil, nil
	}

	var logbuf bytes.Buffer
	log.SetOutput(io.MultiWriter(&logbuf, os.Stdout))

	stop, q := testPreparation(-1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("msg", time.Now()))
	time.Sleep(10 * time.Millisecond)
	q.send()

	foundUnknownErr := false
	loglines := strings.Split(strings.TrimSpace(logbuf.String()), "\n")
	for _, logline := range loglines {
		if strings.Contains(logline, "E!") && strings.Contains(logline, "Unknown Error") {
			foundUnknownErr = true
		}
	}

	require.True(t, foundUnknownErr, fmt.Sprintf("Expecting error log with unknown error, but received '%s' in the log", logbuf.String()))

	log.SetOutput(os.Stderr)

	close(stop)
	wg.Wait()
}

func TestLogRejectedLogEntryInfo(t *testing.T) {
	var wg sync.WaitGroup
	var s stubLogsService

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return &cloudwatchlogs.PutLogEventsOutput{
			RejectedLogEventsInfo: &cloudwatchlogs.RejectedLogEventsInfo{
				TooOldLogEventEndIndex:   aws.Int64(100),
				TooNewLogEventStartIndex: aws.Int64(200),
				ExpiredLogEventEndIndex:  aws.Int64(300),
			},
		}, nil
	}

	var logbuf bytes.Buffer
	log.SetOutput(io.MultiWriter(&logbuf, os.Stdout))

	stop, q := testPreparation(-1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	q.AddEvent(newStubLogEvent("msg", time.Now()))
	time.Sleep(10 * time.Millisecond)
	q.send()

	loglines := strings.Split(strings.TrimSpace(logbuf.String()), "\n")
	require.Len(t, loglines, 4, fmt.Sprintf("Expecting 3 error logs, but %d received", len(loglines)))

	logline := loglines[0]
	require.True(t, strings.Contains(logline, "W!"), fmt.Sprintf("Expecting error log events too old, but received '%s' in the log", logbuf.String()))
	require.True(t, strings.Contains(logline, "100"), fmt.Sprintf("Expecting error log events too old, but received '%s' in the log", logbuf.String()))

	logline = loglines[1]
	require.True(t, strings.Contains(logline, "W!"), fmt.Sprintf("Expecting error log events too new, but received '%s' in the log", logbuf.String()))
	require.True(t, strings.Contains(logline, "200"), fmt.Sprintf("Expecting error log events too new, but received '%s' in the log", logbuf.String()))

	logline = loglines[2]
	require.True(t, strings.Contains(logline, "W!"), fmt.Sprintf("Expecting error log events too expired, but received '%s' in the log", logbuf.String()))
	require.True(t, strings.Contains(logline, "300"), fmt.Sprintf("Expecting error log events too expired, but received '%s' in the log", logbuf.String()))

	log.SetOutput(os.Stderr)

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
	stop, q := testPreparation(-1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
	q.flushTimeout = 50 * time.Millisecond
	q.resetFlushTimer()
	time.Sleep(200 * time.Millisecond) // Wait until pusher started, merge channel is blocked

	for _, e := range evts {
		q.AddEventNonBlocking(e)
	}

	time.Sleep(time.Second)

	close(stop)
	wg.Wait()
}

func TestResendWouldStopAfterExhaustedRetries(t *testing.T) {
	var wg sync.WaitGroup
	var s stubLogsService

	cnt := 0
	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		cnt++
		return nil, &cloudwatchlogs.ServiceUnavailableException{}
	}

	var logbuf bytes.Buffer
	log.SetOutput(io.MultiWriter(&logbuf, os.Stdout))

	stop, q := testPreparation(-1, &s, 10*time.Millisecond, time.Second, nil, &wg)
	q.AddEvent(newStubLogEvent("msg", time.Now()))
	time.Sleep(2 * time.Second)

	loglines := strings.Split(strings.TrimSpace(logbuf.String()), "\n")
	lastline := loglines[len(loglines)-1]
	expected := fmt.Sprintf("All %v retries to G/S failed for PutLogEvents, request dropped.", cnt-1)
	require.True(t, strings.HasSuffix(lastline, expected), fmt.Sprintf("Expecting error log to end with request dropped, but received '%s' in the log", logbuf.String()))

	log.SetOutput(os.Stderr)

	close(stop)
	wg.Wait()
}

func testPreparation(
	retention int,
	service *stubLogsService,
	flushTimeout time.Duration,
	retryDuration time.Duration,
	entityProvider logs.LogEntityProvider,
	wg *sync.WaitGroup,
) (chan struct{}, *queue) {
	stop := make(chan struct{})
	logger := testutil.Logger{Name: "test"}
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
