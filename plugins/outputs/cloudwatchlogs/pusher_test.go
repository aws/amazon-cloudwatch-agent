// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

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
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/influxdata/telegraf/models"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

var wg sync.WaitGroup

type svcMock struct {
	ple func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error)
	clg func(input *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error)
	cls func(input *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error)
	prp func(input *cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error)
}

func (s *svcMock) PutLogEvents(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
	if s.ple != nil {
		return s.ple(in)
	}
	return nil, nil
}
func (s *svcMock) CreateLogGroup(in *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	if s.clg != nil {
		return s.clg(in)
	}
	return nil, nil
}
func (s *svcMock) CreateLogStream(in *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	if s.cls != nil {
		return s.cls(in)
	}
	return nil, nil
}
func (s *svcMock) PutRetentionPolicy(in *cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
	if s.prp != nil {
		return s.prp(in)
	}
	return nil, nil
}

func TestNewPusher(t *testing.T) {
	var s svcMock
	stop, p := testPreparation(-1, &s, time.Second, maxRetryTimeout)

	require.Equal(t, &s, p.Service, "Pusher service does not match the service passed in")
	require.Equal(t, p.Group, "G", fmt.Sprintf("Pusher initialized with the wrong target: %v", p.Target))
	require.Equal(t, p.Stream, "S", fmt.Sprintf("Pusher initialized with the wrong target: %v", p.Target))

	close(stop)
	wg.Wait()
}

type evtMock struct {
	m string
	t time.Time
	d func()
}

func (e evtMock) Message() string { return e.m }
func (e evtMock) Time() time.Time { return e.t }
func (e evtMock) Done() {
	if e.d != nil {
		e.d()
	}
}

func TestAddSingleEvent(t *testing.T) {
	var s svcMock
	called := false
	nst := "NEXT_SEQ_TOKEN"

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		called = true

		if in.SequenceToken != nil {
			t.Errorf("PutLogEvents called with wrong sequenceToken, first call should not provide any token")
		}

		if *in.LogGroupName != "G" || *in.LogStreamName != "S" {
			t.Errorf("PutLogEvents called with wrong group and stream: %v/%v", *in.LogGroupName, *in.LogStreamName)
		}

		if len(in.LogEvents) != 1 || *in.LogEvents[0].Message != "MSG" {
			t.Errorf("PutLogEvents called with incorrect message, got: '%v'", *in.LogEvents[0].Message)
		}

		return &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: &nst,
		}, nil
	}

	stop, p := testPreparation(-1, &s, 1*time.Hour, maxRetryTimeout)

	p.AddEvent(evtMock{"MSG", time.Now(), nil})
	require.False(t, called, "PutLogEvents has been called too fast, it should wait until FlushTimeout.")

	p.FlushTimeout = 10 * time.Millisecond
	p.resetFlushTimer()

	time.Sleep(3 * time.Second)
	require.True(t, called, "PutLogEvents has not been called after FlushTimeout has been reached.")
	require.NotNil(t, nst, *p.sequenceToken, "Pusher did not capture the NextSequenceToken")

	close(stop)
	wg.Wait()
}

func TestStopPusherWouldDoFinalSend(t *testing.T) {
	var s svcMock
	called := false
	nst := "NEXT_SEQ_TOKEN"

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		called = true
		if len(in.LogEvents) != 1 {
			t.Errorf("PutLogEvents called with incorrect number of message, expecting 1, but %v received", len(in.LogEvents))
		}
		return &cloudwatchlogs.PutLogEventsOutput{NextSequenceToken: &nst}, nil
	}

	stop, p := testPreparation(-1, &s, 1*time.Hour, maxRetryTimeout)

	p.AddEvent(evtMock{"MSG", time.Now(), nil})
	time.Sleep(10 * time.Millisecond)

	require.False(t, called, "PutLogEvents has been called too fast, it should wait until FlushTimeout.")

	close(stop)
	wg.Wait()

	require.True(t, called, "PutLogEvents has not been called after FlushTimeout has been reached.")
}

func TestStopPusherWouldStopRetries(t *testing.T) {
	var s svcMock

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return nil, &cloudwatchlogs.ServiceUnavailableException{}
	}

	stop, p := testPreparation(-1, &s, 1*time.Hour, maxRetryTimeout)
	p.AddEvent(evtMock{"MSG", time.Now(), nil})

	sendComplete := make(chan struct{})

	go func() {
		defer close(sendComplete)
		p.send()
	}()

	close(stop)

	select {
	case <-time.After(50 * time.Millisecond):
		t.Errorf("send did not quit retrying after p has been Stopped.")
	case <-sendComplete:
	}
}

func TestLongMessageGetsTruncated(t *testing.T) {
	var s svcMock
	nst := "NEXT_SEQ_TOKEN"
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

		return &cloudwatchlogs.PutLogEventsOutput{NextSequenceToken: &nst}, nil
	}

	stop, p := testPreparation(-1, &s, 1*time.Hour, maxRetryTimeout)
	p.AddEvent(evtMock{longMsg, time.Now(), nil})
	time.Sleep(10 * time.Millisecond)
	p.send()
	close(stop)
	wg.Wait()
}

func TestRequestIsLessThan1MB(t *testing.T) {
	var s svcMock
	nst := "NEXT_SEQ_TOKEN"
	longMsg := strings.Repeat("x", msgSizeLimit)

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {

		length := 0
		for _, le := range in.LogEvents {
			length += len(*le.Message) + eventHeaderSize
		}

		if length > 1024*1024 {
			t.Fatalf("PutLogEvents called with payload larger than request limit of 1MB, %v received", length)
		}

		return &cloudwatchlogs.PutLogEventsOutput{NextSequenceToken: &nst}, nil
	}

	stop, p := testPreparation(-1, &s, 1*time.Hour, maxRetryTimeout)
	for i := 0; i < 8; i++ {
		p.AddEvent(evtMock{longMsg, time.Now(), nil})
	}
	time.Sleep(10 * time.Millisecond)
	p.send()
	p.send()
	close(stop)
	wg.Wait()
}

func TestRequestIsLessThan10kEvents(t *testing.T) {
	var s svcMock
	nst := "NEXT_SEQ_TOKEN"
	msg := "m"

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {

		if len(in.LogEvents) > 10000 {
			t.Fatalf("PutLogEvents called with more than 10k events, %v received", len(in.LogEvents))
		}

		return &cloudwatchlogs.PutLogEventsOutput{NextSequenceToken: &nst}, nil
	}

	stop, p := testPreparation(-1, &s, 1*time.Hour, maxRetryTimeout)
	for i := 0; i < 30000; i++ {
		p.AddEvent(evtMock{msg, time.Now(), nil})
	}
	time.Sleep(10 * time.Millisecond)
	for i := 0; i < 5; i++ {
		p.send()
	}
	close(stop)
	wg.Wait()
}

func TestTimestampPopulation(t *testing.T) {
	var s svcMock
	nst := "NEXT_SEQ_TOKEN"

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {

		if len(in.LogEvents) > 10000 {
			t.Fatalf("PutLogEvents called with more than 10k events, %v received", len(in.LogEvents))
		}

		return &cloudwatchlogs.PutLogEventsOutput{NextSequenceToken: &nst}, nil
	}

	stop, p := testPreparation(-1, &s, 1*time.Hour, maxRetryTimeout)
	for i := 0; i < 3; i++ {
		p.AddEvent(evtMock{"msg", time.Time{}, nil}) // time.Time{} creates zero time
	}
	time.Sleep(10 * time.Millisecond)
	for i := 0; i < 5; i++ {
		p.send()
	}
	close(stop)
	wg.Wait()
}

func TestIgnoreOutOfTimeRangeEvent(t *testing.T) {
	var s svcMock
	nst := "NEXT_SEQ_TOKEN"

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		t.Errorf("PutLogEvents should not be called for out of range events")
		return &cloudwatchlogs.PutLogEventsOutput{NextSequenceToken: &nst}, nil
	}

	var logbuf bytes.Buffer
	log.SetOutput(io.MultiWriter(&logbuf, os.Stdout))

	stop, p := testPreparation(-1, &s, 10*time.Millisecond, maxRetryTimeout)
	p.AddEvent(evtMock{"MSG", time.Now().Add(-15 * 24 * time.Hour), nil})
	p.AddEvent(evtMock{"MSG", time.Now().Add(2*time.Hour + 1*time.Minute), nil})

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
	var s svcMock
	nst := "NEXT_SEQ_TOKEN"

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

		return &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: &nst,
		}, nil
	}

	var evts []evtMock
	start := time.Now().Add(-100 * time.Millisecond)
	for i := 0; i < 100; i++ {
		e := evtMock{
			fmt.Sprintf("MSG - %v", i),
			start.Add(time.Duration(i) * time.Millisecond),
			nil,
		}
		evts = append(evts, e)
	}
	evts[10], evts[90] = evts[90], evts[10] // make events out of order
	stop, p := testPreparation(-1, &s, 1*time.Hour, maxRetryTimeout)
	for _, e := range evts {
		p.AddEvent(e)
	}

	p.FlushTimeout = 10 * time.Millisecond
	p.resetFlushTimer()

	time.Sleep(3 * time.Second)
	require.NotNil(t, p.sequenceToken, "Pusher did not capture the NextSequenceToken")
	require.Equal(t, nst, *p.sequenceToken, "Pusher did not capture the NextSequenceToken")

	close(stop)
	wg.Wait()
}

func TestSendReqWhenEventsSpanMoreThan24Hrs(t *testing.T) {
	var s svcMock
	nst := "NEXT_SEQ_TOKEN"

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
			return &cloudwatchlogs.PutLogEventsOutput{NextSequenceToken: &nst}, nil
		} else if ci == 1 {
			if *in.SequenceToken != nst {
				t.Errorf("PutLogEvents called without correct sequenceToken")
			}

			if len(in.LogEvents) != 1 {
				t.Errorf("PutLogEvents called with incorrect number of message, expecting 1, but %v received", len(in.LogEvents))
			}

			le := in.LogEvents[0]
			if *le.Message != "MSG now" {
				t.Errorf("PutLogEvents received wrong message: '%v'", *le.Message)
			}
			return &cloudwatchlogs.PutLogEventsOutput{NextSequenceToken: &nst}, nil
		}

		t.Errorf("PutLogEvents should not be call more the 2 times")
		return nil, nil
	}

	stop, p := testPreparation(-1, &s, 1*time.Hour, maxRetryTimeout)
	p.AddEvent(evtMock{"MSG 25hrs ago", time.Now().Add(-25 * time.Hour), nil})
	p.AddEvent(evtMock{"MSG 24hrs ago", time.Now().Add(-24 * time.Hour), nil})
	p.AddEvent(evtMock{"MSG 23hrs ago", time.Now().Add(-23 * time.Hour), nil})
	p.AddEvent(evtMock{"MSG now", time.Now(), nil})
	p.FlushTimeout = 10 * time.Millisecond
	p.resetFlushTimer()
	time.Sleep(20 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestUnhandledErrorWouldNotResend(t *testing.T) {
	var s svcMock
	nst := "NEXT_SEQ_TOKEN"

	cnt := 0
	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if cnt == 0 {
			cnt++
			return nil, errors.New("unhandled error")
		}
		t.Errorf("Pusher should not attempt a resend when an unhandled error has been returned")
		return &cloudwatchlogs.PutLogEventsOutput{NextSequenceToken: &nst}, nil
	}

	var logbuf bytes.Buffer
	log.SetOutput(io.MultiWriter(&logbuf, os.Stdout))

	stop, p := testPreparation(-1, &s, 1*time.Hour, maxRetryTimeout)
	p.AddEvent(evtMock{"msg", time.Now(), nil})
	p.FlushTimeout = 10 * time.Millisecond
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
	var s svcMock
	nst := "NEXT_SEQ_TOKEN"

	var plec, clgc, clsc int
	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		var e error
		switch plec {
		case 0:
			e = &cloudwatchlogs.ResourceNotFoundException{}
		case 1:
			e = &cloudwatchlogs.InvalidSequenceTokenException{
				Message_:              aws.String("Invalid SequenceToken"),
				ExpectedSequenceToken: &nst,
			}
		case 2:
			e = awserr.New("Unknown Error", "", nil)
		case 3:
			return &cloudwatchlogs.PutLogEventsOutput{NextSequenceToken: &nst}, nil
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

	stop, p := testPreparation(-1, &s, 1*time.Hour, maxRetryTimeout)
	p.AddEvent(evtMock{"msg", time.Now(), nil})
	time.Sleep(10 * time.Millisecond)
	p.send()

	foundInvalidSeqToken, foundUnknownErr := false, false
	loglines := strings.Split(strings.TrimSpace(logbuf.String()), "\n")
	for _, logline := range loglines {
		if (strings.Contains(logline, "W!") || strings.Contains(logline, "I!")) && strings.Contains(logline, "Invalid SequenceToken") {
			foundInvalidSeqToken = true
		}
		if strings.Contains(logline, "E!") && strings.Contains(logline, "Unknown Error") {
			foundUnknownErr = true
		}
	}

	require.True(t, foundInvalidSeqToken, fmt.Sprintf("Expecting error log with Invalid SequenceToken, but received '%s' in the log", logbuf.String()))
	require.True(t, foundUnknownErr, fmt.Sprintf("Expecting error log with unknown error, but received '%s' in the log", logbuf.String()))

	log.SetOutput(os.Stderr)

	close(stop)
	wg.Wait()
}

func TestCreateLogGroupWithError(t *testing.T) {
	var s svcMock
	stop, p := testPreparation(-1, &s, 1*time.Hour, maxRetryTimeout)

	// test normal case. 1. creating stream fails, 2, creating group succeeds, 3, creating stream succeeds.
	var cnt_clg int
	var cnt_cls int
	s.clg = func(in *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
		cnt_clg++
		return nil, nil
	}
	s.cls = func(in *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
		cnt_cls++
		if cnt_cls == 1 {
			return nil, awserr.New(cloudwatchlogs.ErrCodeResourceNotFoundException, "", nil)
		}

		if cnt_cls == 2 {
			return nil, nil
		}

		t.Errorf("CreateLogStream should not be called when CreateLogGroup failed.")
		return nil, nil
	}

	p.createLogGroupAndStream()

	require.Equal(t, 1, cnt_clg, "CreateLogGroup was not called.")
	require.Equal(t, 2, cnt_cls, "CreateLogStream was not called.")

	// test creating stream succeeds
	cnt_clg = 0
	cnt_cls = 0
	s.clg = func(in *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
		cnt_clg++
		return nil, awserr.New(cloudwatchlogs.ErrCodeResourceAlreadyExistsException, "", nil)
	}
	s.cls = func(in *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
		cnt_cls++
		return nil, nil
	}

	p.createLogGroupAndStream()

	require.Equal(t, 1, cnt_cls, "CreateLogSteam was not called after CreateLogGroup returned ResourceAlreadyExistsException.")
	require.Equal(t, 0, cnt_clg, "CreateLogGroup should not be called when logstream is created successfully at first time.")

	// test creating group fails
	cnt_clg = 0
	cnt_cls = 0
	s.clg = func(in *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
		cnt_clg++
		return nil, awserr.New(cloudwatchlogs.ErrCodeOperationAbortedException, "", nil)
	}
	s.cls = func(in *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
		cnt_cls++
		return nil, awserr.New(cloudwatchlogs.ErrCodeResourceNotFoundException, "", nil)
	}

	err := p.createLogGroupAndStream()
	require.Error(t, err, "createLogGroupAndStream should return err.")

	awsErr, ok := err.(awserr.Error)
	require.False(t, ok && awsErr.Code() != cloudwatchlogs.ErrCodeOperationAbortedException, "createLogGroupAndStream should return ErrCodeOperationAbortedException.")

	require.Equal(t, 1, cnt_cls, "CreateLogSteam should be called for one time.")
	require.Equal(t, 1, cnt_clg, "CreateLogGroup should be called for one time.")

	close(stop)
	wg.Wait()
}

func TestLogRejectedLogEntryInfo(t *testing.T) {
	var s svcMock
	nst := "NEXT_SEQ_TOKEN"

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		return &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: &nst,
			RejectedLogEventsInfo: &cloudwatchlogs.RejectedLogEventsInfo{
				TooOldLogEventEndIndex:   aws.Int64(100),
				TooNewLogEventStartIndex: aws.Int64(200),
				ExpiredLogEventEndIndex:  aws.Int64(300),
			},
		}, nil
	}

	var logbuf bytes.Buffer
	log.SetOutput(io.MultiWriter(&logbuf, os.Stdout))

	stop, p := testPreparation(-1, &s, 1*time.Hour, maxRetryTimeout)
	p.AddEvent(evtMock{"msg", time.Now(), nil})
	time.Sleep(10 * time.Millisecond)
	p.send()

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
	var s svcMock
	nst := "NEXT_SEQ_TOKEN"
	const N = 100

	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if len(in.LogEvents) != N {
			t.Errorf("PutLogEvents called with incorrect number of message, only %v received", len(in.LogEvents))
		}

		return &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: &nst,
		}, nil
	}

	var evts []evtMock
	start := time.Now().Add(-N * time.Millisecond)
	for i := 0; i < N; i++ {
		e := evtMock{
			fmt.Sprintf("MSG - %v", i),
			start.Add(time.Duration(i) * time.Millisecond),
			nil,
		}
		evts = append(evts, e)
	}
	stop, p := testPreparation(-1, &s, 1*time.Hour, maxRetryTimeout)
	p.FlushTimeout = 50 * time.Millisecond
	p.resetFlushTimer()
	time.Sleep(200 * time.Millisecond) // Wait until pusher started, merge channel is blocked

	for _, e := range evts {
		p.AddEventNonBlocking(e)
	}

	time.Sleep(3 * time.Second)
	require.NotNil(t, p.sequenceToken, "Pusher did not capture the NextSequenceToken")
	require.NotNil(t, nst, *p.sequenceToken, "Pusher did not capture the NextSequenceToken")

	close(stop)
	wg.Wait()
}

func TestPutRetentionNegativeInput(t *testing.T) {
	var s svcMock
	var prpc int
	s.prp = func(in *cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
		prpc++
		return nil, nil
	}
	stop, p := testPreparation(-1, &s, 1*time.Hour, maxRetryTimeout)
	p.putRetentionPolicy()

	require.NotEqual(t, 1, prpc, "Put Retention Policy api shouldn't have been called")

	close(stop)
	wg.Wait()
}

func TestPutRetentionValidMaxInput(t *testing.T) {
	var s svcMock
	var prpc = 0
	s.prp = func(in *cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
		prpc++
		return nil, nil
	}
	stop, p := testPreparation(1000000000000000000, &s, 1*time.Hour, maxRetryTimeout)
	p.putRetentionPolicy()

	require.Equal(t, 2, prpc, fmt.Sprintf("Put Retention Policy api should have been called twice. Number of times called: %v", prpc))

	close(stop)
	wg.Wait()
}

func TestPutRetentionWhenError(t *testing.T) {
	var s svcMock
	var prpc int
	s.prp = func(in *cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
		prpc++
		return nil, awserr.New(cloudwatchlogs.ErrCodeResourceNotFoundException, "", nil)

	}
	var logbuf bytes.Buffer
	log.SetOutput(io.MultiWriter(&logbuf, os.Stdout))

	stop, p := testPreparation(1, &s, 1*time.Hour, maxRetryTimeout)
	time.Sleep(10 * time.Millisecond)

	loglines := strings.Split(strings.TrimSpace(logbuf.String()), "\n")
	logline := loglines[0]

	require.NotEqual(t, 0, prpc, fmt.Sprintf("Put Retention Policy should have been called on creation with retention of %v", p.Retention))
	require.True(t, strings.Contains(logline, "ResourceNotFound"), fmt.Sprintf("Expecting ResourceNotFoundException but got '%s' in the log", logbuf.String()))

	close(stop)
	wg.Wait()
}
func TestResendWouldStopAfterExhaustedRetries(t *testing.T) {
	var s svcMock

	cnt := 0
	s.ple = func(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		cnt++
		return nil, &cloudwatchlogs.ServiceUnavailableException{}
	}

	var logbuf bytes.Buffer
	log.SetOutput(io.MultiWriter(&logbuf, os.Stdout))

	stop, p := testPreparation(-1, &s, 10*time.Millisecond, time.Second)
	p.AddEvent(evtMock{"msg", time.Now(), nil})
	time.Sleep(4 * time.Second)

	loglines := strings.Split(strings.TrimSpace(logbuf.String()), "\n")
	lastline := loglines[len(loglines)-1]
	expected := fmt.Sprintf("All %v retries to G/S failed for PutLogEvents, request dropped.", cnt-1)
	require.True(t, strings.HasSuffix(lastline, expected), fmt.Sprintf("Expecting error log to end with request dropped, but received '%s' in the log", logbuf.String()))

	log.SetOutput(os.Stderr)

	close(stop)
	wg.Wait()
}

func testPreparation(retention int, s *svcMock, flushTimeout time.Duration, retryDuration time.Duration) (chan struct{}, *pusher) {
	stop := make(chan struct{})
	p := NewPusher(Target{"G", "S", util.StandardLogGroupClass, retention}, s, flushTimeout, retryDuration, models.NewLogger("cloudwatchlogs", "test", ""), stop, &wg)
	return stop, p
}
