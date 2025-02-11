// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/influxdata/telegraf"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

const (
	retentionChannel = 100
	// max wait time with backoff and jittering:
	// 0 + 2.4 + 4.8 + 9.6 + 10 ~= 26.8 sec
	baseDelay    = 1 * time.Second  // Increased to 1s
	maxDelay     = 10 * time.Second // Keep at 10s
	maxAttempts  = 5                // Keep at 5
	jitterFactor = 0.2
)

type Target struct {
	Group, Stream, Class string
	Retention            int
}

type TargetManager interface {
	InitTarget(target Target) error
	PutRetentionPolicy(target Target)
}

type targetManager struct {
	logger  telegraf.Logger
	service cloudWatchLogsService
	// cache of initialized targets
	cache                  map[Target]struct{}
	mu                     sync.Mutex
	retentionPolicyUpdates chan Target
}

func NewTargetManager(logger telegraf.Logger, service cloudWatchLogsService) TargetManager {
	tm := &targetManager{
		logger:                 logger,
		service:                service,
		cache:                  make(map[Target]struct{}),
		retentionPolicyUpdates: make(chan Target, retentionChannel),
	}

	go tm.processRetentionUpdates()
	return tm
}

// InitTarget initializes a Target if it hasn't been initialized before.
func (m *targetManager) InitTarget(target Target) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.cache[target]; !ok {
		err := m.createLogGroupAndStream(target)
		if err != nil {
			return err
		}
		m.PutRetentionPolicy(target)
		m.cache[target] = struct{}{}
	}
	return nil
}

func (m *targetManager) PutRetentionPolicy(target Target) {
	if target.Retention > 0 {
		// use blocking channel send to not drop any updates when it's full
		m.retentionPolicyUpdates <- target
	}
}

func (m *targetManager) createLogGroupAndStream(t Target) error {
	err := m.createLogStream(t)
	if err == nil {
		return nil
	}

	m.logger.Debugf("creating stream fail due to : %v", err)
	if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == cloudwatchlogs.ErrCodeResourceNotFoundException {
		err = m.createLogGroup(t)

		// attempt to create stream again if group created successfully.
		if err == nil {
			m.logger.Debugf("successfully created log group %v. Retrying log stream %v", t.Group, t.Stream)
			err = m.createLogStream(t)
		} else {
			m.logger.Debugf("creating group fail due to : %v", err)
		}
	}

	if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == cloudwatchlogs.ErrCodeResourceAlreadyExistsException {
		m.logger.Debugf("Resource was already created. %v\n", err)
		return nil // if the log group or log stream already exist, this is not worth returning an error for
	}

	return err
}

func (m *targetManager) createLogGroup(t Target) error {
	var err error
	if t.Class != "" {
		_, err = m.service.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
			LogGroupName:  &t.Group,
			LogGroupClass: &t.Class,
		})
	} else {
		_, err = m.service.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
			LogGroupName: &t.Group,
		})
	}
	return err
}

func (m *targetManager) createLogStream(t Target) error {
	_, err := m.service.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  &t.Group,
		LogStreamName: &t.Stream,
	})

	if err == nil {
		m.logger.Debugf("successfully created log stream %v", t.Stream)
		return nil
	}
	return err
}

func (m *targetManager) processRetentionUpdates() {
	for target := range m.retentionPolicyUpdates {
		err := m.processWithRetry(target)
		if err != nil {
			m.logger.Errorf("Failed to update retention policy for target %v: %v", target, err)
		}
	}
}

func (m *targetManager) processWithRetry(target Target) error {
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err = m.checkAndUpdate(target)
		if err == nil {
			return nil
		}

		m.logger.Debugf("retrying to update retention policy for target (%v) %v: %v", attempt, target, err)
		delay := m.calculateBackoff(attempt)
		time.Sleep(delay)
	}
	return fmt.Errorf("failed to update retention policy after %d attempts: %w", maxAttempts, err)
}

func (m *targetManager) checkAndUpdate(target Target) error {
	currentRetention, err := m.describeLogGroupRetention(target)
	if err != nil {
		return err
	}

	if currentRetention != target.Retention {
		m.logger.Debugf("updating retention policy for target %v: %v", target, err)
		return m.updateRetentionPolicy(target)
	}

	return nil
}

func (m *targetManager) describeLogGroupRetention(target Target) (int, error) {
	input := &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(target.Group),
	}

	output, err := m.service.DescribeLogGroups(input)
	if err != nil {
		return 0, fmt.Errorf("describe log groups failed: %w", err)
	}

	for _, group := range output.LogGroups {
		if *group.LogGroupName == target.Group {
			if group.RetentionInDays == nil {
				return 0, nil
			}
			return int(*group.RetentionInDays), nil
		}
	}

	return 0, fmt.Errorf("log group %v not found", target.Group)
}

func (m *targetManager) updateRetentionPolicy(target Target) error {
	input := &cloudwatchlogs.PutRetentionPolicyInput{
		LogGroupName:    aws.String(target.Group),
		RetentionInDays: aws.Int64(int64(target.Retention)),
	}

	_, err := m.service.PutRetentionPolicy(input)
	if err != nil {
		return fmt.Errorf("put retention policy failed: %w", err)
	}

	return nil
}

func (m *targetManager) calculateBackoff(attempt int) time.Duration {
	delay := baseDelay * time.Duration(1<<uint(attempt))
	if delay > maxDelay {
		delay = maxDelay
	}

	jitter := time.Duration(rand.Float64() * float64(delay) * jitterFactor)
	return delay + jitter
}
