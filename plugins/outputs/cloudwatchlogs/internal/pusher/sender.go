// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/influxdata/telegraf"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

type cloudWatchLogsService interface {
	PutLogEvents(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error)
	CreateLogStream(input *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error)
	CreateLogGroup(input *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error)
	PutRetentionPolicy(input *cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error)
	DescribeLogGroups(input *cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
}

type Sender interface {
	Send(*logEventBatch)
	SetRetryDuration(time.Duration)
	RetryDuration() time.Duration
	Stop()
}

type sender struct {
	service            cloudWatchLogsService
	retryDuration      atomic.Value
	targetManager      TargetManager
	logger             telegraf.Logger
	stopCh             chan struct{}
	stopped            bool
	concurrencyEnabled bool
	retryHeap          RetryHeap
}

var _ (Sender) = (*sender)(nil)

func newSender(
	logger telegraf.Logger,
	service cloudWatchLogsService,
	targetManager TargetManager,
	retryDuration time.Duration,
	concurrencyEnabled bool,
	retryHeap RetryHeap,
) Sender {
	s := &sender{
		logger:             logger,
		service:            service,
		targetManager:      targetManager,
		stopCh:             make(chan struct{}),
		stopped:            false,
		concurrencyEnabled: concurrencyEnabled,
		retryHeap:          retryHeap,
	}
	s.retryDuration.Store(retryDuration)
	return s
}

// Send attempts to send a batch of log events to CloudWatch Logs. Will retry failed attempts until it reaches the
// RetryDuration or an unretryable error.
func (s *sender) Send(batch *logEventBatch) {
	if len(batch.events) == 0 {
		return
	}

	// Initialize start time before build()
	batch.initializeStartTime()
	input := batch.build()

	for {
		output, err := s.service.PutLogEvents(input)
		if err == nil {
			if output.RejectedLogEventsInfo != nil {
				info := output.RejectedLogEventsInfo
				if info.TooOldLogEventEndIndex != nil {
					s.logger.Warnf("%d log events for log '%s/%s' are too old", *info.TooOldLogEventEndIndex, batch.Group, batch.Stream)
				}
				if info.TooNewLogEventStartIndex != nil {
					s.logger.Warnf("%d log events for log '%s/%s' are too new", *info.TooNewLogEventStartIndex, batch.Group, batch.Stream)
				}
				if info.ExpiredLogEventEndIndex != nil {
					s.logger.Warnf("%d log events for log '%s/%s' are expired", *info.ExpiredLogEventEndIndex, batch.Group, batch.Stream)
				}
			}
			batch.done()
			s.logger.Debugf("Pusher published %v log events to group: %v stream: %v with size %v KB in %v.", len(batch.events), batch.Group, batch.Stream, batch.bufferedSize/1024, time.Since(batch.startTime))
			return
		}

		var awsErr awserr.Error
		if !errors.As(err, &awsErr) {
			s.logger.Errorf("Non aws error received when sending logs to %v/%v: %v. CloudWatch agent will not retry and logs will be missing!", batch.Group, batch.Stream, err)
			batch.updateState()
			return
		}

		switch e := awsErr.(type) {
		case *cloudwatchlogs.ResourceNotFoundException:
			if targetErr := s.targetManager.InitTarget(batch.Target); targetErr != nil {
				s.logger.Errorf("Unable to create log stream %v/%v: %v", batch.Group, batch.Stream, targetErr)
				break
			}
		case *cloudwatchlogs.InvalidParameterException,
			*cloudwatchlogs.DataAlreadyAcceptedException:
			s.logger.Errorf("%v, will not retry the request", e)
			batch.updateState()
			return
		default:
			s.logger.Errorf("Aws error received when sending logs to %v/%v: %v", batch.Group, batch.Stream, awsErr)
		}

		// Update retry metadata in the batch
		batch.updateRetryMetadata(err)

		// Check if retry would exceed max duration
		totalRetries := batch.retryCountShort + batch.retryCountLong - 1
		if batch.isExpired(s.RetryDuration()) || batch.nextRetryTime.After(batch.startTime.Add(s.RetryDuration())) {
			s.logger.Errorf("All %v retries to %v/%v failed for PutLogEvents, request dropped.", totalRetries, batch.Group, batch.Stream)
			batch.updateState()
			return
		}

		// If concurrency enabled, push to RetryHeap and return
		// Otherwise, continue with existing busy-wait retry behavior
		if s.isConcurrencyEnabled() {
			s.retryHeap.Push(batch)
			batch.fail()
			return
		}

		// Calculate wait time until next retry (synchronous mode)
		wait := time.Until(batch.nextRetryTime)
		if wait < 0 {
			wait = 0
		}

		s.logger.Warnf("Retried %v time, going to sleep %v before retrying.", totalRetries, wait)

		select {
		case <-s.stopCh:
			s.logger.Errorf("Stop requested after %v retries to %v/%v failed for PutLogEvents, request dropped.", totalRetries, batch.Group, batch.Stream)
			batch.updateState()
			return
		case <-time.After(wait):
		}
	}
}

func (s *sender) Stop() {
	if s.stopped {
		return
	}
	close(s.stopCh)
	s.stopped = true
}

// SetRetryDuration sets the maximum duration for retrying failed log sends.
func (s *sender) SetRetryDuration(retryDuration time.Duration) {
	s.retryDuration.Store(retryDuration)
}

// RetryDuration returns the current maximum retry duration.
func (s *sender) RetryDuration() time.Duration {
	return s.retryDuration.Load().(time.Duration)
}

// isConcurrencyEnabled returns whether concurrency mode is enabled and RetryHeap is available.
func (s *sender) isConcurrencyEnabled() bool {
	return s.concurrencyEnabled && s.retryHeap != nil
}
