// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"bytes"
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
	"github.com/aws/amazon-cloudwatch-agent/profiler"
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

// SendOrdered records the call, then sends the batches in order through Send, as the real sender does, so that
// expectations on Send cover the batches of split log events too. Tests that do not check it set it up with Maybe.
func (m *mockSender) SendOrdered(batches []*logEventBatch) {
	m.Called(batches)
	for _, batch := range batches {
		m.Send(batch)
	}
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

	// A message over the per-event limit is not modified either: it is sent as several events that concatenate to it.
	t.Run("OverLimit", func(t *testing.T) {
		var wg sync.WaitGroup
		var s stubLogsService
		overLimitMsg := strings.Repeat("x", maxEventMessageBytes+10000)
		var mu sync.Mutex
		var received strings.Builder
		var requests int
		s.ple = func(_ context.Context, in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
			mu.Lock()
			defer mu.Unlock()
			requests++
			for _, le := range in.LogEvents {
				received.WriteString(*le.Message)
			}
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		}
		q, sender := testPreparation(t, -1, &s, 1*time.Hour, 2*time.Hour, nil, &wg)
		q.AddEvent(newStubLogEvent(overLimitMsg, time.Now()))
		require.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return requests == 2
		}, 5*time.Second, 10*time.Millisecond)
		q.Stop()
		sender.Stop()
		wg.Wait()
		assert.True(t, received.String() == overLimitMsg, "the events concatenate to the message")
	})
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

// profilerStat returns the value of the profiler stat with the given key.
func profilerStat(key string) float64 {
	stats := profiler.Profiler.GetStats()
	// GetStats returns the map the profiler updates, so read it under the profiler lock.
	profiler.Profiler.Lock()
	defer profiler.Profiler.Unlock()
	return stats[key]
}

// countWarns returns the warnings in the log sink that contain substr.
func countWarns(logSink *testutil.LogSink, substr string) []string {
	var warns []string
	for _, line := range logSink.Lines() {
		if strings.Contains(line, "W!") && strings.Contains(line, substr) {
			warns = append(warns, line)
		}
	}
	return warns
}

// oversizedEMFMessage returns an EMF document over the per-event limit, which is truncated rather than split.
func oversizedEMFMessage() string {
	return `{"_aws":{"CloudWatchMetrics":[]},"data":"` + strings.Repeat("x", maxEventMessageBytes) + `"}`
}

// TestReportTruncatedLogEvents checks that each truncated log event is counted, and that a warning is logged for the
// first one but not for one truncated soon after it. Truncated means content was dropped: an EMF message over the
// limit, or a message past maxSplitChunks events. A message that is split without loss is counted as split instead
// (see TestReportSplitLogEvents).
func TestReportTruncatedLogEvents(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	// A log group used by this test only, as the profiler stats are global.
	target := Target{t.Name(), "S", util.StandardLogGroupClass, -1}
	statKey := strings.Join([]string{"cloudwatchlogs", target.Group, truncatedLogEventsStat}, "_")
	splitStatKey := strings.Join([]string{"cloudwatchlogs", target.Group, splitLogEventsStat}, "_")
	// The stats are not cleared between runs of the test, so compare with the value at the start.
	baseStat := profilerStat(statKey)
	baseSplitStat := profilerStat(splitStatKey)

	mockSender := &mockSender{}
	mockSender.On("SendOrdered", mock.Anything).Return().Maybe()
	mockSender.On("Send", mock.AnythingOfType("*pusher.logEventBatch")).Run(func(args mock.Arguments) {
		args.Get(0).(*logEventBatch).done()
	}).Return()
	logSink := testutil.NewLogSink()
	q := newQueue(logSink, target, time.Hour, nil, mockSender, &wg).(*queue)

	maxMessageSize := maxEventPayloadBytes - perEventHeaderBytes
	// Fewer raw bytes than the limit, but counted as invalidUTF8ByteSize bytes each, so it is over the limit. It is
	// split without loss.
	oversized := string(bytes.Repeat([]byte{0xff}, maxMessageSize))
	emf := oversizedEMFMessage()
	// Counted as 9 times the limit, so it needs more than maxSplitChunks events and loses content.
	pastCap := string(bytes.Repeat([]byte{0xff}, 3*maxMessageSize))
	pastCapEvents := flattenStatefulLogEvents(newStatefulLogEvents(time.Now(), pastCap, nil, nil))
	require.Len(t, pastCapEvents, maxSplitChunks)
	pastCapKept := -len(defaultTruncationSuffix)
	for _, e := range pastCapEvents {
		pastCapKept += e.eventBytes - perEventHeaderBytes
	}

	tests := []struct {
		name          string
		message       string
		expectedStat  float64
		expectedSplit float64
		expectedWarns int
	}{
		{name: "Event that fits", message: "small message", expectedStat: 0, expectedWarns: 0},
		{name: "Split without loss", message: oversized, expectedStat: 0, expectedSplit: 1, expectedWarns: 0},
		{name: "First truncated", message: emf, expectedStat: 1, expectedSplit: 1, expectedWarns: 1},
		// Within warnTruncatedLogEventInterval of the first, so it is counted but not logged.
		{name: "Second truncated", message: emf, expectedStat: 2, expectedSplit: 1, expectedWarns: 1},
		// Past the cap: counted as truncated and as split.
		{name: "Truncated past the cap", message: pastCap, expectedStat: 3, expectedSplit: 2, expectedWarns: 1},
		{name: "Event that fits after truncated", message: "small message", expectedStat: 3, expectedSplit: 2, expectedWarns: 1},
	}
	// The time of the first truncated event, which is the one the warning is logged for.
	var firstTruncatedTime time.Time
	for _, tt := range tests {
		done := make(chan struct{})
		eventTime := time.Now()
		if tt.message == emf && firstTruncatedTime.IsZero() {
			firstTruncatedTime = eventTime
		}
		event := newStubLogEvent(tt.message, eventTime)
		// The queue runs the done callback when the batch is sent, so use it to know the event was processed.
		event.done = func() { close(done) }
		q.AddEvent(event)
		require.Eventually(t, func() bool {
			// The event may not have reached the batch yet, so send until it is acknowledged.
			triggerSend(t, q)
			select {
			case <-done:
				return true
			default:
				return false
			}
		}, 5*time.Second, 10*time.Millisecond, tt.name)

		assert.Equal(t, baseStat+tt.expectedStat, profilerStat(statKey), tt.name)
		assert.Equal(t, baseSplitStat+tt.expectedSplit, profilerStat(splitStatKey), tt.name)
		warns := countWarns(logSink, "truncated")
		require.Len(t, warns, tt.expectedWarns, tt.name)
		if tt.expectedWarns > 0 {
			expected := fmt.Sprintf("The log entry in (%v/S) with timestamp (%v) has a message counted as %d bytes by CloudWatch Logs (%d raw bytes; each byte that is not valid UTF-8 counts as %d), which exceeds the %d byte limit per log event message (after the agent's %d byte per-event overhead); it looks like embedded metric format, so it was truncated, not split, and the remainder dropped.",
				target.Group, firstTruncatedTime, len(emf), len(emf), invalidUTF8ByteSize, maxMessageSize, perEventHeaderBytes)
			assert.Contains(t, warns[0], expected, tt.name)
		}
	}

	// The past-cap variant of the warning, logged once the interval has passed.
	q.Stop()
	wg.Wait()
	q.lastTruncatedWarnTime = time.Time{}
	pastCapEvent := newStubLogEvent(pastCap, time.Now())
	q.reportTruncated(pastCapEvent, flattenStatefulLogEvents(newStatefulLogEvents(pastCapEvent.Time(), pastCap, nil, nil)))
	warns := countWarns(logSink, "truncated")
	require.Len(t, warns, 1, "the past-cap variant does not say truncated")
	warns = countWarns(logSink, "counted bytes dropped")
	require.Len(t, warns, 1)
	expected := fmt.Sprintf("The log entry in (%v/S) with timestamp (%v) has a message counted as %d bytes by CloudWatch Logs (%d raw bytes; each byte that is not valid UTF-8 counts as %d), which exceeds the %d byte limit per log event message (after the agent's %d byte per-event overhead); the message was sent as %d log events and the remaining %d counted bytes dropped.",
		target.Group, pastCapEvent.Time(), invalidUTF8ByteSize*len(pastCap), len(pastCap), invalidUTF8ByteSize, maxMessageSize, perEventHeaderBytes, maxSplitChunks, invalidUTF8ByteSize*len(pastCap)-pastCapKept)
	assert.Contains(t, warns[0], expected)
	assert.Equal(t, baseStat+4, profilerStat(statKey))
}

// TestReportTruncatedLogEvents_WarningRearms checks that the truncation warning is logged again once
// warnTruncatedLogEventInterval has passed since the last one, and that every call is counted. reportTruncated is
// called directly on a queue whose goroutine is not started, so the test can move lastTruncatedWarnTime back without
// a data race or waiting for the interval.
func TestReportTruncatedLogEvents_WarningRearms(t *testing.T) {
	t.Parallel()
	// A log group used by this test only, as the profiler stats are global.
	target := Target{t.Name(), "S", util.StandardLogGroupClass, -1}
	statKey := strings.Join([]string{"cloudwatchlogs", target.Group, truncatedLogEventsStat}, "_")
	// The stats are not cleared between runs of the test, so compare with the value at the start.
	baseStat := profilerStat(statKey)

	logSink := testutil.NewLogSink()
	q := &queue{target: target, logger: logSink}
	event := newStubLogEvent(oversizedEMFMessage(), time.Now())
	events := flattenStatefulLogEvents(newStatefulLogEvents(event.Time(), event.Message(), nil, nil))
	require.Len(t, events, 1)
	require.True(t, events[0].truncated)
	countAllWarns := func() int { return len(countWarns(logSink, "")) }

	q.reportTruncated(event, events)
	assert.Equal(t, 1, countAllWarns(), "first truncated event")
	q.reportTruncated(event, events)
	assert.Equal(t, 1, countAllWarns(), "within warnTruncatedLogEventInterval of the last warning")
	// A split does not mute the truncation warning, and the truncation warning does not mute the split warning: they
	// have separate timers.
	q.reportSplit(event, event.Time(), 2)
	assert.Equal(t, 2, countAllWarns(), "the split warning has its own timer")
	// Move the last warning back past the interval instead of waiting for it.
	q.lastTruncatedWarnTime = time.Now().Add(-warnTruncatedLogEventInterval - time.Second)
	q.reportTruncated(event, events)
	assert.Equal(t, 3, countAllWarns(), "after warnTruncatedLogEventInterval has passed")
	assert.Equal(t, baseStat+3, profilerStat(statKey))
}

// TestReportSplitLogEvents checks that each log event split into several events is counted, with the number of
// events, and that a warning is logged for the first one but not for one split soon after it, without counting the
// split as truncated.
func TestReportSplitLogEvents(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	// A log group used by this test only, as the profiler stats are global.
	target := Target{t.Name(), "S", util.StandardLogGroupClass, -1}
	key := func(stat string) string { return strings.Join([]string{"cloudwatchlogs", target.Group, stat}, "_") }
	// The stats are not cleared between runs of the test, so compare with the value at the start.
	baseSplit := profilerStat(key(splitLogEventsStat))
	baseChunks := profilerStat(key(splitLogEventChunksStat))
	baseTruncated := profilerStat(key(truncatedLogEventsStat))

	var sent atomic.Int32
	mockSender := &mockSender{}
	mockSender.On("SendOrdered", mock.Anything).Return().Maybe()
	mockSender.On("Send", mock.AnythingOfType("*pusher.logEventBatch")).Run(func(args mock.Arguments) {
		sent.Add(1)
		args.Get(0).(*logEventBatch).done()
	}).Return()
	logSink := testutil.NewLogSink()
	q := newQueue(logSink, target, time.Hour, nil, mockSender, &wg).(*queue)

	maxMessageSize := maxEventPayloadBytes - perEventHeaderBytes
	// Counted as 3 times the limit (all invalid). A chunk holds a whole number of invalid bytes, maxMessageSize/3,
	// so it is split into 4 events.
	oversized := string(bytes.Repeat([]byte{0xff}, maxMessageSize))
	numEvents := len(flattenStatefulLogEvents(newStatefulLogEvents(time.Now(), oversized, nil, nil)))
	require.Equal(t, 4, numEvents)

	tests := []struct {
		name           string
		message        string
		expectedSplit  float64
		expectedChunks float64
		expectedWarns  int
	}{
		{name: "Event that fits", message: "small message", expectedWarns: 0},
		{name: "First split", message: oversized, expectedSplit: 1, expectedChunks: 4, expectedWarns: 1},
		// Within warnSplitLogEventInterval of the first split, so it is counted but not logged.
		{name: "Second split", message: oversized, expectedSplit: 2, expectedChunks: 8, expectedWarns: 1},
		{name: "Event that fits after split", message: "small message", expectedSplit: 2, expectedChunks: 8, expectedWarns: 1},
	}
	var firstSplitTime time.Time
	splits := 0
	for _, tt := range tests {
		done := make(chan struct{})
		eventTime := time.Now()
		if tt.message == oversized {
			splits++
		}
		if tt.message == oversized && firstSplitTime.IsZero() {
			firstSplitTime = eventTime
		}
		event := newStubLogEvent(tt.message, eventTime)
		// The queue runs the done callback when the last batch is sent, so use it to know the event was processed.
		event.done = func() { close(done) }
		q.AddEvent(event)
		require.Eventually(t, func() bool {
			triggerSend(t, q)
			select {
			case <-done:
				return true
			default:
				return false
			}
		}, 5*time.Second, 10*time.Millisecond, tt.name)
		// Each split event is handed to the sender with a single SendOrdered call; an event that fits is not.
		mockSender.AssertNumberOfCalls(t, "SendOrdered", splits)

		assert.Equal(t, baseSplit+tt.expectedSplit, profilerStat(key(splitLogEventsStat)), tt.name)
		assert.Equal(t, baseChunks+tt.expectedChunks, profilerStat(key(splitLogEventChunksStat)), tt.name)
		assert.Equal(t, baseTruncated, profilerStat(key(truncatedLogEventsStat)), tt.name)
		warns := countWarns(logSink, "")
		require.Len(t, warns, tt.expectedWarns, tt.name)
		if tt.expectedWarns > 0 {
			expected := fmt.Sprintf("The log entry in (%v/S) with timestamp (%v) has a message counted as %d bytes by CloudWatch Logs (%d raw bytes; each byte that is not valid UTF-8 counts as %d), which exceeds the %d byte limit per log event message (after the agent's %d byte per-event overhead); it was sent as %d log events with the same timestamp.",
				target.Group, firstSplitTime, invalidUTF8ByteSize*len(oversized), len(oversized), invalidUTF8ByteSize, maxMessageSize, perEventHeaderBytes, numEvents)
			assert.Contains(t, warns[0], expected, tt.name)
		}
	}
	// One batch per small event and one per event of each split.
	assert.EqualValues(t, 2+2*numEvents, sent.Load())

	q.Stop()
	wg.Wait()
	// Each SendOrdered call carried all the events of one split event, one batch each.
	for _, call := range mockSender.Calls {
		if call.Method == "SendOrdered" {
			assert.Len(t, call.Arguments.Get(0).([]*logEventBatch), numEvents)
		}
	}
}

// TestReportSplitLogEvents_WarningRearms checks that the split warning is logged again once warnSplitLogEventInterval
// has passed, and that a truncation warning does not mute it.
func TestReportSplitLogEvents_WarningRearms(t *testing.T) {
	t.Parallel()
	target := Target{t.Name(), "S", util.StandardLogGroupClass, -1}
	logSink := testutil.NewLogSink()
	q := &queue{target: target, logger: logSink}
	event := newStubLogEvent(strings.Repeat("x", 2*maxEventMessageBytes), time.Now())
	splitWarns := func() int { return len(countWarns(logSink, "log events with the same timestamp")) }

	q.reportSplit(event, event.Time(), 2)
	assert.Equal(t, 1, splitWarns())
	q.reportSplit(event, event.Time(), 2)
	assert.Equal(t, 1, splitWarns(), "within warnSplitLogEventInterval of the last warning")
	q.reportTruncated(event, flattenStatefulLogEvents(newStatefulLogEvents(event.Time(), oversizedEMFMessage(), nil, nil)))
	assert.Len(t, countWarns(logSink, "truncated"), 1, "the truncation warning has its own timer")
	q.lastSplitWarnTime = time.Now().Add(-warnSplitLogEventInterval - time.Second)
	q.reportSplit(event, event.Time(), 2)
	assert.Equal(t, 2, splitWarns(), "after warnSplitLogEventInterval has passed")
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

func TestTerminalErrorWouldNotResend(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	var s stubLogsService
	var cnt atomic.Int32

	s.ple = func(context.Context, *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		if cnt.Load() == 0 {
			cnt.Add(1)
			// A request that cannot be serialized will never succeed, so the pusher must not retry it.
			return nil, &smithy.SerializationError{Err: errors.New("unhandled error")}
		}
		t.Errorf("Pusher should not attempt a resend when a terminal error has been returned")
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
		mockSender.On("SendOrdered", mock.Anything).Return().Maybe()
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
			resetTimerCh:    make(chan struct{}, 1),
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
		mockSender.On("SendOrdered", mock.Anything).Return().Maybe()
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
			resetTimerCh:    make(chan struct{}, 1),
			flushTimer:      time.NewTimer(10 * time.Millisecond),
			startNonBlockCh: make(chan struct{}),
			wg:              &wg,
		}
		q.flushTimeout.Store(10 * time.Millisecond)

		event := newStubStatefulLogEvent("test message", time.Now(), state.NewRange(10, 20), mrq)

		convertedSingle, convertedSplit := q.converter.convert(event)
		require.NotNil(t, convertedSingle)
		require.Nil(t, convertedSplit)
		q.batch.append(convertedSingle)

		q.send()

		mockSender.AssertExpectations(t)
	})
}

func TestResetTimerChIsBuffered(t *testing.T) {
	var wg sync.WaitGroup
	var s stubLogsService
	q, sender := testPreparation(t, -1, &s, time.Second, 2*time.Hour, nil, &wg)
	t.Cleanup(func() { q.Stop(); sender.Stop(); wg.Wait() })

	require.Equalf(t, 1, cap(q.resetTimerCh),
		"resetTimerCh must be buffered (cap 1) so a force-flush-timer rearm issued while "+
			"manageFlushTimer is outside its select is parked, not dropped; cap=0 reintroduces "+
			"the lost-wakeup bug")
}

func TestForceFlushTimerRearmsAfterHeldSend(t *testing.T) {
	const flushTimeout = 300 * time.Millisecond
	const flushBound = 3 * flushTimeout

	var (
		sendCount    atomic.Int32
		secondSendAt atomic.Value
		firstEntered = make(chan struct{})
		release      = make(chan struct{})
		releaseOnce  sync.Once
	)
	releaseSend := func() { releaseOnce.Do(func() { close(release) }) }

	var s stubLogsService
	s.ple = func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		switch sendCount.Load() {
		case 0:
			sendCount.Add(1)
			close(firstEntered)
			<-release
		case 1:
			secondSendAt.Store(time.Now())
			sendCount.Add(1)
		}
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	var wg sync.WaitGroup
	q, sender := testPreparation(t, -1, &s, flushTimeout, 2*time.Hour, nil, &wg)
	t.Cleanup(func() { releaseSend(); q.Stop(); sender.Stop(); wg.Wait() })

	q.AddEvent(newStubLogEvent("small", time.Now()))
	time.Sleep(100 * time.Millisecond)

	// The largest message that fits in one event, which fills a request, so the small event is sent first. A larger
	// message would be split and sent at once, leaving no trailing event for the flush timer.
	q.AddEvent(newStubLogEvent(strings.Repeat("X", maxEventMessageBytes), time.Now()))

	select {
	case <-firstEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("size-based send never started; test setup wrong")
	}

	time.Sleep(flushTimeout + 100*time.Millisecond)
	tRelease := time.Now()
	releaseSend()

	deadline := time.Now().Add(flushBound)
	for time.Now().Before(deadline) && sendCount.Load() < 2 {
		time.Sleep(5 * time.Millisecond)
	}

	require.GreaterOrEqual(t, sendCount.Load(), int32(2),
		"trailing event must be sent via force-flush timer within %v after the held send completes", flushBound)

	raw := secondSendAt.Load()
	require.NotNil(t, raw, "secondSendAt never recorded")
	delay := raw.(time.Time).Sub(tRelease)
	require.Less(t, delay, flushBound,
		"trailing event took %v after release; if the flush timer was dead it would never arrive", delay)
}

func TestForceFlushTimer_SingleLowVolumeEventFlushes(t *testing.T) {
	const flushTimeout = 200 * time.Millisecond

	var sendCount atomic.Int32
	var s stubLogsService
	s.ple = func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
		sendCount.Add(1)
		return &cloudwatchlogs.PutLogEventsOutput{}, nil
	}

	var wg sync.WaitGroup
	q, sender := testPreparation(t, -1, &s, flushTimeout, 2*time.Hour, nil, &wg)
	t.Cleanup(func() { q.Stop(); sender.Stop(); wg.Wait() })

	q.AddEvent(newStubLogEvent("single event", time.Now()))

	deadline := time.Now().Add(3 * flushTimeout)
	for time.Now().Before(deadline) && sendCount.Load() < 1 {
		time.Sleep(5 * time.Millisecond)
	}

	require.GreaterOrEqual(t, sendCount.Load(), int32(1),
		"a single low-volume event must be flushed by the timer within %v", 3*flushTimeout)
}

// uniqueMessage returns an ASCII message of at least n bytes made of numbered runs, so that events reassembled out of
// order would not match it, with a space every 8 bytes for the natural cuts.
func uniqueMessage(n int) string {
	var sb strings.Builder
	for i := 0; sb.Len() < n; i++ {
		fmt.Fprintf(&sb, "%07d ", i)
	}
	return sb.String()
}

// splitRecorder is a PutLogEvents stub that records the requests and whether the source range was enqueued before
// the last one.
type splitRecorder struct {
	mu       sync.Mutex
	requests [][]types.InputLogEvent
	// The number of requests made when the range was enqueued, or -1.
	requestsAtEnqueue int
	// The request whose first message is delayMessage is delayed by delay before it is recorded.
	delayMessage string
	delay        time.Duration
	// The request whose first message is failMessage fails with a terminal error.
	failMessage string
}

func newSplitRecorder() *splitRecorder {
	return &splitRecorder{requestsAtEnqueue: -1}
}

func (r *splitRecorder) putLogEvents(_ context.Context, in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
	first := *in.LogEvents[0].Message
	if r.delayMessage != "" && first == r.delayMessage {
		// A slow request: without the ordered sequence, a free worker would send the next chunk meanwhile.
		time.Sleep(r.delay)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, in.LogEvents)
	if r.failMessage != "" && first == r.failMessage {
		return nil, &types.InvalidParameterException{Message: aws.String("rejected")}
	}
	return &cloudwatchlogs.PutLogEventsOutput{}, nil
}

func (r *splitRecorder) onEnqueue(mock.Arguments) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requestsAtEnqueue = len(r.requests)
}

// TestSplitLogEventSentInOrder checks that a message over the per-event limit is sent as an ordered sequence of
// single-event requests, with the source timestamp on every event and the events concatenating to the message; that
// the open batch is sent before them and the next event after them; and that the source range is enqueued once, after
// the last request, with or without a worker pool, including when a request in the middle of the sequence is
// rejected.
func TestSplitLogEventSentInOrder(t *testing.T) {
	message := uniqueMessage(3*maxEventMessageBytes + 1)
	chunks := flattenStatefulLogEvents(newStatefulLogEvents(time.Now(), message, nil, nil))
	require.Len(t, chunks, 4)

	tests := []struct {
		name        string
		concurrency int
		slowFirst   bool
		rejectChunk bool
	}{
		{name: "NoPool"},
		{name: "NoPoolWithSlowFirstChunk", slowFirst: true},
		// The first chunk's request is slow, so with a batch per task a free worker would send the next chunk first.
		{name: "PoolWithSlowFirstChunk", concurrency: 4, slowFirst: true},
		// The second chunk is rejected: it is dropped as any rejected batch is, and the rest is still sent in order.
		{name: "NoPoolWithRejectedChunk", rejectChunk: true},
		{name: "PoolWithRejectedChunk", concurrency: 4, rejectChunk: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := newSplitRecorder()
			if tt.slowFirst {
				rec.delayMessage, rec.delay = chunks[0].message, 300*time.Millisecond
			}
			if tt.rejectChunk {
				rec.failMessage = chunks[1].message
			}
			var s stubLogsService
			s.ple = rec.putLogEvents

			var workerPool WorkerPool
			if tt.concurrency > 1 {
				workerPool = NewWorkerPool(tt.concurrency)
				defer workerPool.Stop()
			}
			logger := testutil.NewNopLogger()
			sender := createSender(logger, &s, NewTargetManager(logger, &s), workerPool, time.Hour)
			var wg sync.WaitGroup
			q := newQueue(logger, Target{"G", "S", util.StandardLogGroupClass, -1}, time.Hour, nil, sender, &wg).(*queue)

			r := state.NewRange(100, 200)
			mrq := &mockRangeQueue{}
			mrq.On("ID").Return("file")
			mrq.On("Enqueue", r).Run(rec.onEnqueue).Return().Once()

			eventTime := time.Now()
			q.AddEvent(newStubLogEvent("before", eventTime.Add(-time.Millisecond)))
			q.AddEvent(newStubStatefulLogEvent(message, eventTime, r, mrq))
			q.AddEvent(newStubLogEvent("after", eventTime.Add(time.Millisecond)))
			// "after" stays in the open batch until it is flushed.
			require.Eventually(t, func() bool {
				rec.mu.Lock()
				defer rec.mu.Unlock()
				return len(rec.requests) == 1+len(chunks)
			}, 5*time.Second, 10*time.Millisecond)
			triggerSend(t, q)
			q.Stop()
			wg.Wait()
			sender.Stop()
			require.Eventually(t, func() bool {
				rec.mu.Lock()
				defer rec.mu.Unlock()
				return len(rec.requests) == 2+len(chunks)
			}, 5*time.Second, 10*time.Millisecond)
			// The range is enqueued after the last chunk's request returns, so wait for it too.
			require.Eventually(t, func() bool {
				rec.mu.Lock()
				defer rec.mu.Unlock()
				return rec.requestsAtEnqueue >= 0
			}, 5*time.Second, 10*time.Millisecond)

			rec.mu.Lock()
			defer rec.mu.Unlock()
			requests := rec.requests
			// With a pool, the open batch is sent by another task, so it may land before or after the sequence; the
			// sequence itself is never interleaved with it.
			var sequence [][]types.InputLogEvent
			for _, req := range requests {
				require.NotEmpty(t, req)
				switch *req[0].Message {
				case "before", "after":
					require.Len(t, req, 1)
				default:
					sequence = append(sequence, req)
				}
			}
			if tt.concurrency == 0 {
				assert.Equal(t, "before", *requests[0][0].Message)
			}
			assert.Equal(t, "after", *requests[len(requests)-1][0].Message)
			require.Len(t, sequence, len(chunks))
			var reassembled strings.Builder
			for i, req := range sequence {
				require.Len(t, req, 1, "each chunk is sent in a request of its own")
				assert.Equal(t, chunks[i].message, *req[0].Message, "chunk %d", i)
				assert.Equal(t, eventTime.UnixMilli(), *req[0].Timestamp, "chunk %d", i)
				reassembled.WriteString(*req[0].Message)
			}
			assert.True(t, reassembled.String() == message, "the chunks concatenate to the message")
			mrq.AssertExpectations(t)
			// The range was enqueued once the last chunk's request had been made, so after every chunk. With a pool,
			// the request of another batch can be made in between.
			lastChunk := 0
			for i, req := range requests {
				if *req[0].Message == chunks[len(chunks)-1].message {
					lastChunk = i
				}
			}
			assert.GreaterOrEqual(t, rec.requestsAtEnqueue, lastChunk+1)
			if tt.concurrency == 0 {
				assert.Equal(t, lastChunk+1, rec.requestsAtEnqueue)
			}
		})
	}
}

// TestSplitLogEventDoneOnce checks that the done callback of a split log event without state runs once, after the
// last chunk's request has completed.
func TestSplitLogEventDoneOnce(t *testing.T) {
	t.Parallel()
	rec := newSplitRecorder()
	var s stubLogsService
	s.ple = rec.putLogEvents
	var wg sync.WaitGroup
	q, sender := testPreparation(t, -1, &s, time.Hour, time.Hour, nil, &wg)

	message := uniqueMessage(2*maxEventMessageBytes + 1)
	var doneCalls atomic.Int32
	var requestsAtDone atomic.Int64
	event := newStubLogEvent(message, time.Now())
	event.done = func() {
		rec.mu.Lock()
		requestsAtDone.Store(int64(len(rec.requests)))
		rec.mu.Unlock()
		doneCalls.Add(1)
	}
	q.AddEvent(event)
	require.Eventually(t, func() bool { return doneCalls.Load() > 0 }, 5*time.Second, 10*time.Millisecond)
	q.Stop()
	sender.Stop()
	wg.Wait()
	assert.EqualValues(t, 1, doneCalls.Load())
	assert.EqualValues(t, 3, requestsAtDone.Load())
}

// TestSplitLogEventEMFNotSplit checks that an EMF message over the limit is sent as one truncated event.
func TestSplitLogEventEMFNotSplit(t *testing.T) {
	t.Parallel()
	rec := newSplitRecorder()
	var s stubLogsService
	s.ple = rec.putLogEvents
	var wg sync.WaitGroup
	q, sender := testPreparation(t, -1, &s, time.Hour, time.Hour, nil, &wg)

	message := oversizedEMFMessage()
	q.AddEventNonBlocking(newStubLogEvent(message, time.Now()))
	require.Eventually(t, func() bool {
		triggerSend(t, q)
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return len(rec.requests) > 0
	}, 5*time.Second, 10*time.Millisecond)
	q.Stop()
	sender.Stop()
	wg.Wait()

	rec.mu.Lock()
	defer rec.mu.Unlock()
	require.Len(t, rec.requests, 1)
	require.Len(t, rec.requests[0], 1)
	expected, _, truncated := validateAndTruncateMessage(message)
	require.True(t, truncated)
	assert.Equal(t, expected, *rec.requests[0][0].Message)
}
