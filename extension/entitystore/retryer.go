// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"go.uber.org/zap"
)

const (
	RequestLimitExceeded = "RequestLimitExceeded"
	infRetry             = -1
)

var (
	retryableErrorMap = map[string]bool{
		"RequestLimitExceeded": true,
	}
)

type Retryer struct {
	oneTime         bool
	retryAnyError   bool
	successRetryMin int
	successRetryMax int
	backoffArray    []time.Duration
	maxRetry        int
	done            chan struct{}
	logger          *zap.Logger
}

func NewRetryer(onetime bool, retryAnyError bool, successRetryMin int, successRetryMax int, backoffArray []time.Duration, maxRetry int, done chan struct{}, logger *zap.Logger) *Retryer {
	return &Retryer{
		oneTime:         onetime,
		retryAnyError:   retryAnyError,
		successRetryMin: successRetryMin,
		successRetryMax: successRetryMax,
		backoffArray:    backoffArray,
		maxRetry:        maxRetry,
		done:            done,
		logger:          logger,
	}
}

func (r *Retryer) refreshLoop(updateFunc func() error) int {
	// Offset retry by 1 so we can start with 1 minute wait time
	// instead of immediately retrying
	retry := 1
	for {
		if r.maxRetry != -1 && retry > r.maxRetry {
			return retry
		}
		err := updateFunc()
		if err == nil && r.oneTime {
			return retry
		} else if awsErr, ok := err.(awserr.Error); ok && !r.retryAnyError && !retryableErrorMap[awsErr.Code()] {
			return retry
		}

		waitDuration := calculateWaitTime(retry-1, err, r.successRetryMin, r.successRetryMax, r.backoffArray)
		wait := time.NewTimer(waitDuration)
		select {
		case <-r.done:
			r.logger.Debug("Shutting down retryer")
			wait.Stop()
			return retry
		case <-wait.C:
		}

		if retry > 1 {
			r.logger.Debug("attribute retrieval retry count", zap.Int("retry", retry-1))
		}

		if err != nil {
			retry++
			r.logger.Debug("there was an issue when retrieving entity attributes but will not affect agent functionality", zap.Error(err))
		} else {
			retry = 1
		}

	}
	return retry
}

// calculateWaitTime returns different time based on whether if
// a function call was returned with error. If returned with error,
// follow exponential backoff wait time, otherwise, refresh with jitter
func calculateWaitTime(retry int, err error, successRetryMin int, successRetryMax int, backoffArray []time.Duration) time.Duration {
	var waitDuration time.Duration
	if err == nil {
		return time.Duration(rand.Intn(successRetryMax-successRetryMin)+successRetryMin) * time.Second
	}
	if retry < len(backoffArray) {
		waitDuration = backoffArray[retry]
	} else {
		waitDuration = backoffArray[len(backoffArray)-1]
	}
	return waitDuration
}
