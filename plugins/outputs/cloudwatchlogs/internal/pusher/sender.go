// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"context"
	"errors"
	"sync"
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
	Stop()
}

type sender struct {
	service       cloudWatchLogsService
	targetManager TargetManager
	logger        telegraf.Logger
	stopCh        chan struct{}
	stopOnce      sync.Once
	retryHeap     RetryHeap
}

var _ (Sender) = (*sender)(nil)

func newSender(
	logger telegraf.Logger,
	service cloudWatchLogsService,
	targetManager TargetManager,
	retryHeap RetryHeap,
) Sender {
	s := &sender{
		logger:        logger,
		service:       service,
		targetManager: targetManager,
		stopCh:        make(chan struct{}),
		retryHeap:     retryHeap,
	}
	return s
}

// Send attempts to send a batch of log events to CloudWatch Logs. Retries failed attempts until the batch
// expires (batch.isExpired, governed by expireAfter) or an unretryable error is returned.
func (s *sender) Send(batch *logEventBatch) {
	if len(batch.events) == 0 {
		return
	}

	// Initialize start time before build()
	batch.initializeStartTime()
	input := batch.build()

	// Pin the batch to the client that first sent it so retries keep any per-destination
	// client state, then always send through that client.
	if batch.service == nil {
		batch.service = s.service
	}
	service := batch.service

	ctx := context.Background()
	for {
		output, err := service.PutLogEvents(ctx, input)
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

		var apiErr smithy.APIError
		if !errors.As(err, &apiErr) {
			s.logger.Errorf("Non aws error received when sending logs to %v/%v: %v. CloudWatch agent will not retry and logs will be missing!", batch.Group, batch.Stream, err)
			batch.drop()
			return
		}

		var resourceNotFound *types.ResourceNotFoundException
		var invalidParameter *types.InvalidParameterException
		var dataAlreadyAccepted *types.DataAlreadyAcceptedException
		switch {
		case errors.As(err, &resourceNotFound):
			if targetErr := s.targetManager.InitTarget(batch.Target); targetErr != nil {
				s.logger.Errorf("Unable to create log stream %v/%v: %v", batch.Group, batch.Stream, targetErr)
				break
			}
		case errors.As(err, &invalidParameter) || errors.As(err, &dataAlreadyAccepted):
			s.logger.Errorf("%v, will not retry the request", err)
			batch.drop()
			return
		default:
			s.logger.Errorf("Aws error received when sending logs to %v/%v: %v", batch.Group, batch.Stream, apiErr)
		}

		// Update retry metadata in the batch
		batch.updateRetryMetadata(err)

		// Check if retry would exceed max duration
		totalRetries := batch.retryCountShort + batch.retryCountLong - 1
		if batch.isExpired() {
			s.logger.Errorf("All %v retries to %v/%v failed for PutLogEvents, request dropped.", totalRetries, batch.Group, batch.Stream)
			batch.drop()
			return
		}

		// If RetryHeap available, push to RetryHeap and return
		// Otherwise, continue with existing busy-wait retry behavior
		if s.retryHeap != nil {
			if err := s.retryHeap.Push(batch); err != nil {
				// Heap closed because shutdown is in progress. This is transient, so
				// state is NOT persisted: the events are re-read after restart rather
				// than being reported as delivered and silently lost.
				s.logger.Warnf("RetryHeap stopped, abandoning batch for %v/%v (will be re-read after restart): %v", batch.Group, batch.Stream, err)
				batch.abandon()
				return
			}
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
			// drop(), not abandon(): persisting state here is deliberate so the batch is not
			// reprocessed after restart, trading loss for no duplication on shutdown.
			s.logger.Errorf("Stop requested after %v retries to %v/%v failed for PutLogEvents, request dropped.", totalRetries, batch.Group, batch.Stream)
			batch.drop()
			return
		case <-time.After(wait):
		}
	}
}

// Stop is idempotent: shutdown signals the pusher once before destination locks are taken
// (see cwDest.signalStop) and again on the normal Stop path.
func (s *sender) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}
