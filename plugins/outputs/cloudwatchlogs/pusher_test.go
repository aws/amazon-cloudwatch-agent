package cloudwatchlogs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/influxdata/telegraf/models"
)

type svcMock struct {
	ple func(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error)
	clg func(input *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error)
	cls func(input *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error)
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

func TestNewPusher(t *testing.T) {
	var s svcMock
	p := NewPusher(Target{"G", "S"}, &s, time.Second, maxRetryTimeout, models.NewLogger("cloudwatchlogs", "test", ""))
	if p.Service != &s {
		t.Errorf("Pusher service does not match the service passed in")
	}

	if p.Group != "G" || p.Stream != "S" {
		t.Errorf("Pusher initialized with the wrong target: %v", p.Target)
	}

	p.Stop()
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

	p := NewPusher(Target{"G", "S"}, &s, 1*time.Hour, maxRetryTimeout, models.NewLogger("cloudwatchlogs", "test", ""))
	p.AddEvent(evtMock{"MSG", time.Now(), nil})

	if called {
		t.Errorf("PutLogEvents has been called too fast, it should wait until FlushTimeout.")
	}
	p.FlushTimeout = 10 * time.Millisecond
	p.resetFlushTimer()
	time.Sleep(2000 * time.Millisecond)
	if !called {
		t.Errorf("PutLogEvents has not been called after FlushTimeout has been reached.")
	}

	if *p.sequenceToken != nst {
		t.Errorf("Pusher did not capture the NextSequenceToken")
	}
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

	p := NewPusher(Target{"G", "S"}, &s, 1*time.Hour, maxRetryTimeout, models.NewLogger("cloudwatchlogs", "test", ""))
	p.AddEvent(evtMock{"MSG", time.Now(), nil})

	if called {
		t.Errorf("PutLogEvents has been called too fast, it should wait until FlushTimeout.")
	}
	time.Sleep(10 * time.Millisecond)
	p.Stop()
	time.Sleep(10 * time.Millisecond)
	if !called {
		t.Errorf("PutLogEvents has not been called after p has been Stopped.")
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

	p := NewPusher(Target{"G", "S"}, &s, 1*time.Hour, maxRetryTimeout, models.NewLogger("cloudwatchlogs", "test", ""))
	p.AddEvent(evtMock{longMsg, time.Now(), nil})
	time.Sleep(10 * time.Millisecond)
	p.send()
	p.Stop()
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

	p := NewPusher(Target{"G", "S"}, &s, 1*time.Hour, maxRetryTimeout, models.NewLogger("cloudwatchlogs", "test", ""))
	for i := 0; i < 8; i++ {
		p.AddEvent(evtMock{longMsg, time.Now(), nil})
	}
	time.Sleep(10 * time.Millisecond)
	p.send()
	p.send()
	p.Stop()
	time.Sleep(10 * time.Millisecond)
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

	p := NewPusher(Target{"G", "S"}, &s, 1*time.Hour, maxRetryTimeout, models.NewLogger("cloudwatchlogs", "test", ""))
	for i := 0; i < 30000; i++ {
		p.AddEvent(evtMock{msg, time.Now(), nil})
	}
	time.Sleep(10 * time.Millisecond)
	for i := 0; i < 5; i++ {
		p.send()
	}
	p.Stop()
	time.Sleep(100 * time.Millisecond)
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

	p := NewPusher(Target{"G", "S"}, &s, 1*time.Hour, maxRetryTimeout, models.NewLogger("cloudwatchlogs", "test", ""))
	for i := 0; i < 3; i++ {
		p.AddEvent(evtMock{"msg", time.Time{}, nil}) // time.Time{} creates zero time
	}
	time.Sleep(10 * time.Millisecond)
	for i := 0; i < 5; i++ {
		p.send()
	}
	p.Stop()
	time.Sleep(100 * time.Millisecond)
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

	p := NewPusher(Target{"G", "S"}, &s, 10*time.Millisecond, maxRetryTimeout, models.NewLogger("cloudwatchlogs", "test", ""))
	p.AddEvent(evtMock{"MSG", time.Now().Add(-15 * 24 * time.Hour), nil})
	p.AddEvent(evtMock{"MSG", time.Now().Add(2*time.Hour + 1*time.Minute), nil})

	loglines := strings.Split(strings.TrimSpace(logbuf.String()), "\n")
	if len(loglines) != 2 {
		t.Errorf("Expecting 2 error logs, but %d received", len(loglines))
	}
	for _, logline := range loglines {
		if !strings.Contains(logline, "E!") || !strings.Contains(logline, "Discard the log entry") {
			t.Errorf("Expecting error log with unhandled error, but received '%s' in the log", logbuf.String())
		}
	}

	log.SetOutput(os.Stderr)

	time.Sleep(20 * time.Millisecond)
	p.Stop()
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
	p := NewPusher(Target{"G", "S"}, &s, 1*time.Hour, maxRetryTimeout, models.NewLogger("cloudwatchlogs", "test", ""))
	for _, e := range evts {
		p.AddEvent(e)
	}

	p.FlushTimeout = 10 * time.Millisecond
	p.resetFlushTimer()
	time.Sleep(2000 * time.Millisecond)

	if p.sequenceToken == nil || *p.sequenceToken != nst {
		t.Errorf("Pusher did not capture the NextSequenceToken")
	}
	p.Stop()
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

	p := NewPusher(Target{"G", "S"}, &s, 1*time.Hour, maxRetryTimeout, models.NewLogger("cloudwatchlogs", "test", ""))
	p.AddEvent(evtMock{"MSG 25hrs ago", time.Now().Add(-25 * time.Hour), nil})
	p.AddEvent(evtMock{"MSG 24hrs ago", time.Now().Add(-24 * time.Hour), nil})
	p.AddEvent(evtMock{"MSG 23hrs ago", time.Now().Add(-23 * time.Hour), nil})
	p.AddEvent(evtMock{"MSG now", time.Now(), nil})
	p.FlushTimeout = 10 * time.Millisecond
	p.resetFlushTimer()
	time.Sleep(20 * time.Millisecond)
	p.Stop()
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

	p := NewPusher(Target{"G", "S"}, &s, 1*time.Hour, maxRetryTimeout, models.NewLogger("cloudwatchlogs", "test", ""))
	p.AddEvent(evtMock{"msg", time.Now(), nil})
	p.FlushTimeout = 10 * time.Millisecond
	time.Sleep(2000 * time.Millisecond)

	logline := logbuf.String()
	if !strings.Contains(logline, "E!") || !strings.Contains(logline, "unhandled error") {
		t.Errorf("Expecting error log with unhandled error, but received '%s' in the log", logbuf.String())
	}
	log.SetOutput(os.Stderr)

	p.Stop()

	if cnt != 1 {
		t.Errorf("Expecting pusher to call send 1 time, but %d times called", cnt)
	}
	time.Sleep(20 * time.Millisecond)
}

func TestCreateLogGroupAndLogSteamWhenNotFound(t *testing.T) {
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

	p := NewPusher(Target{"G", "S"}, &s, 1*time.Hour, maxRetryTimeout, models.NewLogger("cloudwatchlogs", "test", ""))
	p.AddEvent(evtMock{"msg", time.Now(), nil})
	time.Sleep(10 * time.Millisecond)
	p.send()

	foundInvalidSeqToken, foundUnknownErr := false, false
	loglines := strings.Split(strings.TrimSpace(logbuf.String()), "\n")
	for _, logline := range loglines {
		if strings.Contains(logline, "W!") && strings.Contains(logline, "Invalid SequenceToken") {
			foundInvalidSeqToken = true
		}
		if strings.Contains(logline, "E!") && strings.Contains(logline, "Unknown Error") {
			foundUnknownErr = true
		}
	}
	if !foundInvalidSeqToken {
		t.Errorf("Expecting error log with Invalid SequenceToken, but received '%s' in the log", logbuf.String())
	}
	if !foundUnknownErr {
		t.Errorf("Expecting error log with unknown error, but received '%s' in the log", logbuf.String())
	}

	log.SetOutput(os.Stderr)

	p.Stop()
	time.Sleep(10 * time.Millisecond)
}

func TestCreateLogGroupWithError(t *testing.T) {
	var s svcMock
	p := NewPusher(Target{"G", "S"}, &s, 1*time.Hour, maxRetryTimeout, models.NewLogger("cloudwatchlogs", "test", ""))

	var cnt int
	s.clg = func(in *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
		cnt++
		return nil, errors.New("Any Error")
	}
	s.cls = func(in *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
		t.Errorf("CreateLogStream should not be called when CreateLogGroup failed.")
		return nil, nil
	}

	p.createLogGroupAndStream()

	if cnt != 1 {
		t.Errorf("CreateLogGroup was not called.")
	}

	cnt = 0
	s.clg = func(in *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
		return nil, awserr.New(cloudwatchlogs.ErrCodeResourceAlreadyExistsException, "", nil)
	}
	s.cls = func(in *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
		cnt++
		return nil, nil
	}

	p.createLogGroupAndStream()

	if cnt != 1 {
		t.Errorf("CreateLogSteam was not called after CreateLogGroup returned ResourceAlreadyExistsException.")
	}

	p.Stop()
	time.Sleep(10 * time.Millisecond)

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

	p := NewPusher(Target{"G", "S"}, &s, 1*time.Hour, maxRetryTimeout, models.NewLogger("cloudwatchlogs", "test", ""))
	p.AddEvent(evtMock{"msg", time.Now(), nil})
	time.Sleep(10 * time.Millisecond)
	p.send()

	loglines := strings.Split(strings.TrimSpace(logbuf.String()), "\n")
	if len(loglines) != 4 { // 3 warnings and 1 debug
		t.Errorf("Expecting 3 error logs, but %d received", len(loglines))
	}
	logline := loglines[0]
	if !strings.Contains(logline, "W!") || !strings.Contains(logline, "100") {
		t.Errorf("Expecting error log events too old, but received '%s' in the log", logbuf.String())
	}
	logline = loglines[1]
	if !strings.Contains(logline, "W!") || !strings.Contains(logline, "200") {
		t.Errorf("Expecting error log events too new, but received '%s' in the log", logbuf.String())
	}
	logline = loglines[2]
	if !strings.Contains(logline, "W!") || !strings.Contains(logline, "300") {
		t.Errorf("Expecting error log events expired, but received '%s' in the log", logbuf.String())
	}

	log.SetOutput(os.Stderr)

	p.Stop()
	time.Sleep(10 * time.Millisecond)
}
