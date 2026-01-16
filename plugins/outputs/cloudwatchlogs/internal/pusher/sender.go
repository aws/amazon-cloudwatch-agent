// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/smithy-go"
	"github.com/influxdata/telegraf"
)

type cloudWatchLogsService interface {
	PutLogEvents(ctx context.Context, input *cloudwatchlogs.PutLogEventsInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error)
	CreateLogStream(ctx context.Context, input *cloudwatchlogs.CreateLogStreamInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error)
	CreateLogGroup(ctx context.Context, input *cloudwatchlogs.CreateLogGroupInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error)
	PutRetentionPolicy(ctx context.Context, input *cloudwatchlogs.PutRetentionPolicyInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error)
	DescribeLogGroups(ctx context.Context, input *cloudwatchlogs.DescribeLogGroupsInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
}

type Sender interface {
	Send(*logEventBatch)
	SetRetryDuration(time.Duration)
	RetryDuration() time.Duration
	Stop()
}

type sender struct {
	service       cloudWatchLogsService
	retryDuration atomic.Value
	targetManager TargetManager
	logger        telegraf.Logger
	stopCh        chan struct{}
	stopped       bool
}

var _ (Sender) = (*sender)(nil)

func newSender(
	logger telegraf.Logger,
	service cloudWatchLogsService,
	targetManager TargetManager,
	retryDuration time.Duration,
) Sender {
	s := &sender{
		logger:        logger,
		service:       service,
		targetManager: targetManager,
		stopCh:        make(chan struct{}),
		stopped:       false,
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
	input := batch.build()
	startTime := time.Now()

	retryCountShort := 0
	retryCountLong := 0
	ctx := context.Background()
	for {
		output, err := s.service.PutLogEvents(ctx, input)
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
			s.logger.Debugf("Pusher published %v log events to group: %v stream: %v with size %v KB in %v.", len(batch.events), batch.Group, batch.Stream, batch.bufferedSize/1024, time.Since(startTime))
			return
		}

		var resourceNotFound *types.ResourceNotFoundException
		if errors.As(err, &resourceNotFound) {
			if targetErr := s.targetManager.InitTarget(batch.Target); targetErr != nil {
				s.logger.Errorf("Unable to create log stream %v/%v: %v", batch.Group, batch.Stream, targetErr)
			}
		} else {
			var invalidParam *types.InvalidParameterException
			var dataAlreadyAccepted *types.DataAlreadyAcceptedException
			if errors.As(err, &invalidParam) || errors.As(err, &dataAlreadyAccepted) {
				s.logger.Errorf("%v, will not retry the request", err)
				batch.updateState()
				return
			}
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				s.logger.Errorf("Aws error received when sending logs to %v/%v: %v", batch.Group, batch.Stream, apiErr)
			} else {
				s.logger.Errorf("Non aws error received when sending logs to %v/%v: %v. CloudWatch agent will not retry and logs will be missing!", batch.Group, batch.Stream, err)
				batch.updateState()
				return
			}
		}

		// retry wait strategy depends on the type of error returned
		var wait time.Duration
		if chooseRetryWaitStrategy(err) == retryLong {
			wait = retryWaitLong(retryCountLong)
			retryCountLong++
		} else {
			wait = retryWaitShort(retryCountShort)
			retryCountShort++
		}

		if time.Since(startTime)+wait > s.RetryDuration() {
			s.logger.Errorf("All %v retries to %v/%v failed for PutLogEvents, request dropped.", retryCountShort+retryCountLong-1, batch.Group, batch.Stream)
			batch.updateState()
			return
		}

		s.logger.Warnf("Retried %v time, going to sleep %v before retrying.", retryCountShort+retryCountLong-1, wait)

		select {
		case <-s.stopCh:
			s.logger.Errorf("Stop requested after %v retries to %v/%v failed for PutLogEvents, request dropped.", retryCountShort+retryCountLong-1, batch.Group, batch.Stream)
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
