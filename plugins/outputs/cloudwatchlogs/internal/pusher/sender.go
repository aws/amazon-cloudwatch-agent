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
}

type sender struct {
	service       cloudWatchLogsService
	retryDuration atomic.Value
	targetManager TargetManager
	logger        telegraf.Logger
	stop          <-chan struct{}
}

func newSender(
	logger telegraf.Logger,
	service cloudWatchLogsService,
	targetManager TargetManager,
	retryDuration time.Duration,
	stop <-chan struct{},
) Sender {
	s := &sender{
		logger:        logger,
		service:       service,
		targetManager: targetManager,
		stop:          stop,
	}
	s.retryDuration.Store(retryDuration)
	return s
}

// Send attempts to send a batch of log events to CloudWatch Logs. Will retry failed attempts until it reaches the
// RetryDuration or an unretryable error.
func (s *sender) Send(batch *logEventBatch) {
	if len(batch.events) == 0 {
		s.logger.Debugf("[SEND DEBUG] Empty batch, nothing to send")
		return
	}
	
	// Detailed logging before sending
	s.logger.Infof("[SEND DEBUG] Preparing to send batch to CloudWatch Logs:")
	s.logger.Infof("  - Log Group: %s", batch.Group)
	s.logger.Infof("  - Log Stream: %s", batch.Stream)
	s.logger.Infof("  - Number of events: %d", len(batch.events))
	s.logger.Infof("  - Total batch size: %d bytes (%.2f KB)", batch.bufferedSize, float64(batch.bufferedSize)/1024)
	s.logger.Infof("  - Batch size limit: %d bytes (%.2f KB)", reqSizeLimit, float64(reqSizeLimit)/1024)
	s.logger.Infof("  - Events limit: %d", reqEventsLimit)
	
	// Check for truncated messages in the batch
	truncatedCount := 0
	for i, event := range batch.events {
		if event.Message != nil && len(*event.Message) >= 15 {
			if (*event.Message)[len(*event.Message)-15:] == "[Truncated...]" {
				truncatedCount++
				if truncatedCount <= 3 { // Log details for first 3 truncated messages
					s.logger.Warnf("[SEND DEBUG] Truncated message #%d in batch:", truncatedCount)
					s.logger.Warnf("  - Event index: %d", i)
					s.logger.Warnf("  - Message size: %d bytes", len(*event.Message))
					s.logger.Warnf("  - Message preview (last 100 chars): %s", (*event.Message)[max(0, len(*event.Message)-100):])
				}
			}
		}
	}
	
	if truncatedCount > 0 {
		s.logger.Warnf("[SEND DEBUG] Total truncated messages in batch: %d/%d", truncatedCount, len(batch.events))
	}
	
	input := batch.build()
	startTime := time.Now()

	retryCountShort := 0
	retryCountLong := 0
	for {
		output, err := s.service.PutLogEvents(input)
		if err == nil {
			// Success - detailed logging
			s.logger.Infof("[SEND DEBUG] Successfully sent batch to CloudWatch Logs:")
			s.logger.Infof("  - Log Group: %s", batch.Group)
			s.logger.Infof("  - Log Stream: %s", batch.Stream)
			s.logger.Infof("  - Events sent: %d", len(batch.events))
			s.logger.Infof("  - Total size: %d bytes (%.2f KB)", batch.bufferedSize, float64(batch.bufferedSize)/1024)
			s.logger.Infof("  - Duration: %v", time.Since(startTime))
			
			if output.RejectedLogEventsInfo != nil {
				info := output.RejectedLogEventsInfo
				s.logger.Warnf("[SEND DEBUG] Some events were rejected by CloudWatch Logs:")
				if info.TooOldLogEventEndIndex != nil {
					s.logger.Warnf("  - %d log events for log '%s/%s' are too old", *info.TooOldLogEventEndIndex, batch.Group, batch.Stream)
				}
				if info.TooNewLogEventStartIndex != nil {
					s.logger.Warnf("  - %d log events for log '%s/%s' are too new", *info.TooNewLogEventStartIndex, batch.Group, batch.Stream)
				}
				if info.ExpiredLogEventEndIndex != nil {
					s.logger.Warnf("  - %d log events for log '%s/%s' are expired", *info.ExpiredLogEventEndIndex, batch.Group, batch.Stream)
				}
			}
			batch.done()
			s.logger.Debugf("Pusher published %v log events to group: %v stream: %v with size %v KB in %v.", len(batch.events), batch.Group, batch.Stream, batch.bufferedSize/1024, time.Since(startTime))
			return
		}

		var awsErr awserr.Error
		if !errors.As(err, &awsErr) {
			s.logger.Errorf("Non aws error received when sending logs to %v/%v: %v. CloudWatch agent will not retry and logs will be missing!", batch.Group, batch.Stream, err)
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
			return
		default:
			s.logger.Errorf("Aws error received when sending logs to %v/%v: %v", batch.Group, batch.Stream, awsErr)
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
			return
		}

		s.logger.Warnf("Retried %v time, going to sleep %v before retrying.", retryCountShort+retryCountLong-1, wait)

		select {
		case <-s.stop:
			s.logger.Errorf("Stop requested after %v retries to %v/%v failed for PutLogEvents, request dropped.", retryCountShort+retryCountLong-1, batch.Group, batch.Stream)
			return
		case <-time.After(wait):
		}
	}
}

// SetRetryDuration sets the maximum duration for retrying failed log sends.
func (s *sender) SetRetryDuration(retryDuration time.Duration) {
	s.retryDuration.Store(retryDuration)
}

// RetryDuration returns the current maximum retry duration.
func (s *sender) RetryDuration() time.Duration {
	return s.retryDuration.Load().(time.Duration)
}
