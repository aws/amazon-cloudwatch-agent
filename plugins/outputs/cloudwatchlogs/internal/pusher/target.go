// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/influxdata/telegraf"

	"github.com/aws/amazon-cloudwatch-agent/sdk/service/cloudwatchlogs"
)

type Target struct {
	Group, Stream, Class string
	Retention            int
}

type TargetManager interface {
	InitTarget(target *Target) error
	PutRetentionPolicy(target *Target)
}

type targetManager struct {
	logger  telegraf.Logger
	service cloudWatchLogsService
	// cache of initialized targets
	cache map[*Target]struct{}
	mu    sync.Mutex
}

func NewTargetManager(logger telegraf.Logger, service cloudWatchLogsService) TargetManager {
	return &targetManager{
		logger:  logger,
		service: service,
		cache:   make(map[*Target]struct{}),
	}
}

// InitTarget initializes a Target if it hasn't been initialized before.
func (m *targetManager) InitTarget(target *Target) error {
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

func (m *targetManager) createLogGroupAndStream(t *Target) error {
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

func (m *targetManager) createLogGroup(t *Target) error {
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

func (m *targetManager) createLogStream(t *Target) error {
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

// PutRetentionPolicy tries to set the retention policy for a log group. Does not retry on failure.
func (m *targetManager) PutRetentionPolicy(t *Target) {
	if t.Retention > 0 {
		i := aws.Int64(int64(t.Retention))
		putRetentionInput := &cloudwatchlogs.PutRetentionPolicyInput{
			LogGroupName:    &t.Group,
			RetentionInDays: i,
		}
		_, err := m.service.PutRetentionPolicy(putRetentionInput)
		if err != nil {
			// since this gets called both before we start pushing logs, and after we first attempt
			// to push a log to a non-existent log group, we don't want to dirty the log with an error
			// if the error is that the log group doesn't exist (yet).
			if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == cloudwatchlogs.ErrCodeResourceNotFoundException {
				m.logger.Debugf("Log group %v not created yet: %v", t.Group, err)
			} else {
				m.logger.Errorf("Unable to put retention policy for log group %v: %v ", t.Group, err)
			}
		} else {
			m.logger.Debugf("successfully updated log retention policy for log group %v", t.Group)
		}
	}
}
