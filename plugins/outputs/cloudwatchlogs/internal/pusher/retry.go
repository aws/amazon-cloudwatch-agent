// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"errors"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

const (
	// baseRetryDelayShort is the base retry delay for short retry strategy
	baseRetryDelayShort = 200 * time.Millisecond

	// baseRetryDelayLong is the base retry delay for long retry strategy
	baseRetryDelayLong = 2 * time.Second

	// numBackoffRetriesShort is the maximum number of consecutive retries using the short retry strategy before using
	// the maxRetryDelay
	numBackoffRetriesShort = 5

	// numBackoffRetriesLong is the maximum number of consecutive retries using the long retry strategy before using
	// the maxRetryDelay
	numBackoffRetriesLong = 2

	// maxRetryDelay is the maximum retry delay for the either retry strategy
	maxRetryDelay = 1 * time.Minute
)

type retryWaitStrategy int

const (
	retryShort retryWaitStrategy = iota
	retryLong
)

// retryWaitShort returns a duration to wait before retrying a request using the short retry strategy
func retryWaitShort(retryCount int) time.Duration {
	return retryWait(baseRetryDelayShort, numBackoffRetriesShort, retryCount)
}

// retryWaitLong returns a duration to wait before retrying a request using the long retry strategy.
// this strategy is used for errors that should not be retried too quickly
func retryWaitLong(retryCount int) time.Duration {
	return retryWait(baseRetryDelayLong, numBackoffRetriesLong, retryCount)
}

func retryWait(baseRetryDelay time.Duration, maxBackoffRetries int, retryCount int) time.Duration {
	d := maxRetryDelay
	if retryCount < maxBackoffRetries {
		d = baseRetryDelay * time.Duration(1<<int64(retryCount))
	}
	return withJitter(d)
}

func withJitter(d time.Duration) time.Duration {
	return time.Duration(rand.Int63n(int64(d/2)) + int64(d/2)) // nolint:gosec
}

// chooseRetryWaitStrategy decides if a "long" or "short" retry strategy should be used when the PutLogEvents API call
// returns an error. A short retry strategy should be used for most errors, while a long retry strategy is used for
// errors where retrying too quickly could cause excessive strain on the backend servers.
//
// Specifically, use the long retry strategy for the following PutLogEvents errors:
//   - 500 (InternalFailure)
//   - 503 (ServiceUnavailable)
//   - Connection Refused
//   - Connection Reset by Peer
//   - Connection Timeout
//   - Throttling
func chooseRetryWaitStrategy(err error) retryWaitStrategy {
	if isErrConnectionTimeout(err) || isErrConnectionReset(err) || isErrConnectionRefused(err) || request.IsErrorThrottle(err) {
		return retryLong
	}

	// Check AWS Error codes if available
	var awsErr awserr.Error
	if errors.As(err, &awsErr) {
		switch awsErr.Code() {
		case
			cloudwatchlogs.ErrCodeServiceUnavailableException,
			cloudwatchlogs.ErrCodeThrottlingException,
			"RequestTimeout",
			request.ErrCodeResponseTimeout:
			return retryLong
		}

		// Check HTTP status codes if available
		var requestFailure awserr.RequestFailure
		if errors.As(err, &requestFailure) {
			switch requestFailure.StatusCode() {
			case
				500, // internal failure
				503: // service unavailable
				return retryLong
			}
		}
	}

	// Otherwise, default to short retry strategy
	return retryShort
}

func isErrConnectionTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func isErrConnectionReset(err error) bool {
	errStr := err.Error()
	if strings.Contains(errStr, "read: connection reset") {
		return false
	}

	return strings.Contains(errStr, "use of closed network connection") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe")
}

func isErrConnectionRefused(err error) bool {
	return strings.Contains(err.Error(), "connection refused")
}
