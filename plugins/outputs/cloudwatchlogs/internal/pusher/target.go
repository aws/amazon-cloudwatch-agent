// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/influxdata/telegraf"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

const (
	retentionChannelSize = 100
	// max wait time with backoff and jittering:
	// 0 + 2.4 + 4.8 + 9.6 + 10 ~= 26.8 sec
	baseRetryDelay      = 1 * time.Second
	maxRetryDelayTarget = 10 * time.Second
	numBackoffRetries   = 5
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
	cache map[Target]struct{}
	mu    sync.Mutex
	dlg   chan Target
	prp   chan Target
}

func NewTargetManager(logger telegraf.Logger, service cloudWatchLogsService) TargetManager {
	tm := &targetManager{
		logger:  logger,
		service: service,
		cache:   make(map[Target]struct{}),
		dlg:     make(chan Target, retentionChannelSize),
		prp:     make(chan Target, retentionChannelSize),
	}

	go tm.processDescribeLogGroup()
	go tm.processPutRetentionPolicy()
	return tm
}

// InitTarget initializes a Target if it hasn't been initialized before.
func (m *targetManager) InitTarget(target Target) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.cache[target]; !ok {
		newGroup, err := m.createLogGroupAndStream(target)
		if err != nil {
			return err
		}
		if target.Retention > 0 {
			if newGroup {
				m.logger.Debugf("sending new log group %v to prp channel", target.Group)
				m.prp <- target
			} else {
				m.logger.Debugf("sending existing log group %v to dlg channel", target.Group)
				m.dlg <- target
			}
		}
		m.cache[target] = struct{}{}
	}
	return nil
}

func (m *targetManager) PutRetentionPolicy(target Target) {
	// new pusher will call this so start with dlg
	if target.Retention > 0 {
		m.logger.Debugf("sending log group %v to dlg channel by pusher", target.Group)
		m.dlg <- target
	}
}

func (m *targetManager) createLogGroupAndStream(t Target) (bool, error) {
	err := m.createLogStream(t)
	if err == nil {
		return false, nil
	}

	m.logger.Debugf("creating stream fail due to : %v", err)
	newGroup := false
	if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == cloudwatchlogs.ErrCodeResourceNotFoundException {
		err = m.createLogGroup(t)
		newGroup = true

		// attempt to create stream again if group created successfully.
		if err == nil {
			m.logger.Debugf("retrying log stream %v", t.Stream)
			err = m.createLogStream(t)
		} else {
			m.logger.Debugf("creating group fail due to : %v", err)
		}
	}

	if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == cloudwatchlogs.ErrCodeResourceAlreadyExistsException {
		m.logger.Debugf("resource was already created. %v\n", err)
		return false, nil
	}

	return newGroup, err
}

func (m *targetManager) createLogGroup(t Target) error {
	var input *cloudwatchlogs.CreateLogGroupInput
	if t.Class != "" {
		input = &cloudwatchlogs.CreateLogGroupInput{
			LogGroupName:  &t.Group,
			LogGroupClass: &t.Class,
		}
	} else {
		input = &cloudwatchlogs.CreateLogGroupInput{
			LogGroupName: &t.Group,
		}
	}
	_, err := m.service.CreateLogGroup(input)
	if err == nil {
		m.logger.Debugf("successfully created log group %v", t.Group)
		return nil
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

func (m *targetManager) processDescribeLogGroup() {
	for target := range m.dlg {
		for attempt := 0; attempt < numBackoffRetries; attempt++ {
			currentRetention, err := m.getRetention(target)
			if err != nil {
				m.logger.Errorf("failed to describe log group retention for target %v: %v", target, err)
				time.Sleep(m.calculateBackoff(attempt))
				continue
			}

			if currentRetention != target.Retention && target.Retention > 0 {
				m.logger.Debugf("queueing log group %v to update retention policy", target.Group)
				m.prp <- target
			}
			break // no change in retention
		}
	}
}

func (m *targetManager) getRetention(target Target) (int, error) {
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

func (m *targetManager) processPutRetentionPolicy() {
	for target := range m.prp {
		var updated bool
		for attempt := 0; attempt < numBackoffRetries; attempt++ {
			err := m.updateRetentionPolicy(target)
			if err == nil {
				updated = true
				break
			}

			m.logger.Debugf("retrying to update retention policy for target (%v) %v: %v", attempt, target, err)
			time.Sleep(m.calculateBackoff(attempt))
		}

		if !updated {
			m.logger.Errorf("failed to update retention policy for target %v after %d attempts", target, numBackoffRetries)
		}
	}
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
	m.logger.Debugf("successfully updated retention policy for log group %v", target.Group)
	return nil
}

func (m *targetManager) calculateBackoff(retryCount int) time.Duration {
	delay := baseRetryDelay
	if retryCount < numBackoffRetries {
		delay = baseRetryDelay * time.Duration(1<<int64(retryCount))
	}
	if delay > maxRetryDelayTarget {
		delay = maxRetryDelayTarget
	}
	return time.Duration(seededRand.Int63n(int64(delay/2)) + int64(delay/2))
}
