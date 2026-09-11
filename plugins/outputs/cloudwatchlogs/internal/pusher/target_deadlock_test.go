// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
	"github.com/aws/amazon-cloudwatch-agent/tool/testutil"
)

// newThrottlingClient returns a real CloudWatch Logs client whose endpoint points
// at a local server that always responds with a ThrottlingException, wired with a
// LogThrottleRetryer. The returned retryer is also handed back so the test can stop
// its consumer goroutine to reproduce the dead-consumer condition.
func newThrottlingClient(t *testing.T) (*cloudwatchlogs.Client, *retryer.LogThrottleRetryer, func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// JSON 1.1 protocol: the SDK classifies the error from the error type,
		// which it reads from this header / body. ThrottlingException is a
		// throttling error, so the SDK will invoke IsErrorRetryable and retry.
		w.Header().Set("X-Amzn-Errortype", "ThrottlingException")
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"__type":"ThrottlingException","message":"Rate exceeded"}`))
	}))

	// The embedded retry.Standard bounds attempts (default 3, i.e. >1) so a dead
	// consumer fills the capacity-1 throttle channel and (pre-fix) the next send blocks.
	r := retryer.NewLogThrottleRetryer(testutil.NewNopLogger())

	client := cloudwatchlogs.NewFromConfig(aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("ak", "sk", ""),
	}, func(o *cloudwatchlogs.Options) {
		o.BaseEndpoint = aws.String(srv.URL)
		o.Retryer = r
	})

	return client, r, srv.Close
}

// TestInitTargetNoDeadlockUnderThrottling drives the real SDK retry loop through
// InitTarget while CreateLogStream is throttled and the retryer's consumer has been
// stopped. InitTarget holds a mutex across the create call, so a blocking throttle
// send would wedge every target. Asserts both targets' InitTarget return in time.
func TestInitTargetNoDeadlockUnderThrottling(t *testing.T) {
	t.Parallel()
	client, r, closeSrv := newThrottlingClient(t)
	defer closeSrv()

	manager := NewTargetManager(testutil.NewNopLogger(), client)

	// Stop the retryer's consumer BEFORE any calls, reproducing the dead-consumer
	// condition that arises when the destination owning the retryer stops.
	r.Stop()
	time.Sleep(50 * time.Millisecond)

	var wg sync.WaitGroup
	done := make(chan struct{})
	for i, target := range []Target{
		{Group: "group-A", Stream: "stream-A"},
		{Group: "group-B", Stream: "stream-B"},
	} {
		wg.Add(1)
		go func(_ int, tg Target) {
			defer wg.Done()
			// Returns a throttling error after retries are exhausted; the point is
			// that it RETURNS rather than parking forever inside the held mutex.
			_ = manager.InitTarget(tg)
		}(i, target)
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Both InitTarget calls returned: no deadlock.
	case <-time.After(60 * time.Second):
		t.Fatal("InitTarget deadlocked under throttling with a stopped retryer consumer")
	}
}
