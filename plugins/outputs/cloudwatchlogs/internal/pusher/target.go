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
	cacheTTL                      = 5 * time.Second
	logGroupIdentifierLimit       = 50
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
	cache    map[Target]time.Time
	cacheTTL time.Duration
	mu       sync.Mutex
	dlg      chan Target
	prp      chan Target
}

func NewTargetManager(logger telegraf.Logger, service cloudWatchLogsService) TargetManager {
	tm := &targetManager{
		logger:   logger,
		service:  service,
		cache:    make(map[Target]time.Time),
		cacheTTL: cacheTTL,
		dlg:      make(chan Target, retentionChannelSize),
		prp:      make(chan Target, retentionChannelSize),
	}

	go tm.processDescribeLogGroup()
	go tm.processPutRetentionPolicy()
	return tm
}

// InitTarget initializes a Target if it hasn't been initialized before. Stores a timestamp of the last successful
// initialization of the Target. If the timestamp is older than the TTL, allows the creation attempt again.
func (m *targetManager) InitTarget(target Target) error {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	lastHit, ok := m.cache[target]
	if !ok || now.Sub(lastHit) > m.cacheTTL {
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
		m.cache[target] = time.Now()
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
	if m.isLogStreamCreated(err, t.Stream) {
		return false, nil
	}

	m.logger.Debugf("creating stream %v fail due to: %v", t.Stream, err)
	if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == cloudwatchlogs.ErrCodeResourceNotFoundException {
		err = m.createLogGroup(t)

		// attempt to create stream again if group created successfully.
		if m.isLogGroupCreated(err, t.Group) {
			m.logger.Debugf("retrying log stream %v", t.Stream)
			err = m.createLogStream(t)
			if m.isLogStreamCreated(err, t.Stream) {
				return true, nil
			}
		} else {
			m.logger.Debugf("creating group %v fail due to: %v", t.Group, err)
		}
	}

	return false, err
}

func (m *targetManager) isLogGroupCreated(err error, group string) bool {
	return m.isResourceCreated(err, fmt.Sprintf("log group %v", group))
}

func (m *targetManager) isLogStreamCreated(err error, stream string) bool {
	return m.isResourceCreated(err, fmt.Sprintf("log stream %v", stream))
}

func (m *targetManager) isResourceCreated(err error, resourceName string) bool {
	if err == nil {
		return true
	}
	// if the resource already exist
	if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == cloudwatchlogs.ErrCodeResourceAlreadyExistsException {
		m.logger.Debugf("%s was already created. %v\n", resourceName, err)
		return true
	}
	return false
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
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()

	batch := make(map[string]Target, logGroupIdentifierLimit)

	for {
		select {
		case target := <-m.dlg:
			batch[target.Group] = target
			if len(batch) == logGroupIdentifierLimit {
				m.updateTargetBatch(batch)
				// Reset batch
				batch = make(map[string]Target, logGroupIdentifierLimit)
			}
		case <-t.C:
			if len(batch) > 0 {
				m.updateTargetBatch(batch)
				// Reset batch
				batch = make(map[string]Target, logGroupIdentifierLimit)
			}
		}
	}
}

func (m *targetManager) updateTargetBatch(targets map[string]Target) {
	var identifiers []*string
	for logGroup := range targets {
		identifiers = append(identifiers, aws.String(logGroup))
	}
	describeLogGroupsInput := &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupIdentifiers: identifiers,
	}
	for attempt := 0; attempt < numBackoffRetries; attempt++ {
		output, err := m.service.DescribeLogGroups(describeLogGroupsInput)
		if err != nil {
			m.logger.Errorf("failed to describe log group retention for targets %v: %v", targets, err)
			time.Sleep(m.calculateBackoff(attempt))
			continue
		}

		for _, logGroups := range output.LogGroups {
			target := targets[*logGroups.LogGroupName]
			if target.Retention != int(*logGroups.RetentionInDays) && target.Retention > 0 {
				m.logger.Debugf("queueing log group %v to update retention policy", target.Group)
				m.prp <- target
			}
		}
		break
	}
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
	return withJitter(delay)
}
