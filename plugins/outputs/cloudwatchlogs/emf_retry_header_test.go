// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	"github.com/aws/amazon-cloudwatch-agent/tool/util"
)

// emfMessage is shaped so cwDest.Publish detects it as EMF and calls switchToEMF().
const emfMessage = `{"_aws":{"Timestamp":1700000000000,"CloudWatchMetrics":[{"Namespace":"n","Dimensions":[[]],"Metrics":[{"Name":"m"}]}]},"m":1}`

// emfHeaderRecorder is a stub CloudWatch Logs endpoint that records the
// x-amzn-logs-format header of every PutLogEvents attempt.
type emfHeaderRecorder struct {
	mu sync.Mutex
	// emfHeaders holds one entry per PutLogEvents attempt, in order.
	emfHeaders []string
	failFirst  bool
}

func (h *emfHeaderRecorder) attempts() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.emfHeaders...)
}

func (h *emfHeaderRecorder) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		if !strings.Contains(r.Header.Get("X-Amz-Target"), "PutLogEvents") {
			// CreateLogGroup / CreateLogStream / DescribeLogGroups / PutRetentionPolicy.
			io.WriteString(w, `{}`)
			return
		}

		h.mu.Lock()
		h.emfHeaders = append(h.emfHeaders, r.Header.Get("x-amzn-logs-format"))
		n := len(h.emfHeaders)
		h.mu.Unlock()

		if h.failFirst && n == 1 {
			// OperationAbortedException at HTTP 400: the SDK's DefaultRetryer does not
			// retry it, so it surfaces to sender.Send, which pushes the batch onto the
			// retry heap. Any later attempt therefore comes from the heap processor,
			// not from an SDK-internal retry on the originating client.
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"__type":"OperationAbortedException","message":"forced by test"}`)
			return
		}
		io.WriteString(w, `{}`)
	}
}

// TestEMFHeaderPreservedOnRetriedBatch is an end-to-end wire test over the real AWS SDK
// client chain. switchToEMF() installs the x-amzn-logs-format header handler on the
// originating dest's client only. If a retried batch is re-sent through a different
// client, CloudWatch Logs ingests it as plain log events and performs no metric
// extraction -- silent metric loss, since the request itself still succeeds.
func TestEMFHeaderPreservedOnRetriedBatch(t *testing.T) {
	rec := &emfHeaderRecorder{failFirst: true}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	c := &CloudWatchLogs{
		Log:                testutil.Logger{Name: "test"},
		Region:             "us-east-1",
		AccessKey:          "access_key",
		SecretKey:          "secret_key",
		EndpointOverride:   srv.URL,
		Concurrency:        2,
		ForceFlushInterval: internal.Duration{Duration: 100 * time.Millisecond},
		cwDests:            sync.Map{},
	}
	defer c.Close()

	dest := c.CreateDest("G", "S", -1, util.StandardLogGroupClass, nil).(*cwDest)
	require.NotNil(t, c.retryHeapProcessor, "retry heap must be active at concurrency > 1")

	require.NoError(t, dest.Publish([]logs.LogEvent{
		&retryHeapTestEvent{msg: emfMessage, ts: time.Now()},
	}))
	require.True(t, dest.isEMF, "EMF-shaped message must switch the dest to EMF mode")

	// Wait for the forced failure plus at least one heap-driven retry.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) && len(rec.attempts()) < 2 {
		time.Sleep(25 * time.Millisecond)
	}

	got := rec.attempts()
	require.GreaterOrEqual(t, len(got), 2,
		"expected an initial attempt and at least one retry; got %d", len(got))
	require.Equal(t, "json/emf", got[0],
		"first attempt must carry the EMF header (sanity: switchToEMF wired the dest client)")

	for i, h := range got[1:] {
		require.Equal(t, "json/emf", h,
			"retry attempt %d lost the EMF header: the batch was re-sent through a client "+
				"that switchToEMF() never touched, so CloudWatch will not extract metrics", i+2)
	}
}

// TestEMFHeaderPreservedOnRetriedBatchSynchronous is the contrast case. At concurrency <= 1
// there is no retry heap, so sender.Send retries in place on the originating client. This
// documents that the EMF header is only ever at risk on the concurrent path.
func TestEMFHeaderPreservedOnRetriedBatchSynchronous(t *testing.T) {
	rec := &emfHeaderRecorder{failFirst: true}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	c := &CloudWatchLogs{
		Log:                testutil.Logger{Name: "test"},
		Region:             "us-east-1",
		AccessKey:          "access_key",
		SecretKey:          "secret_key",
		EndpointOverride:   srv.URL,
		Concurrency:        0, // default: synchronous sleep-retry, no heap
		ForceFlushInterval: internal.Duration{Duration: 100 * time.Millisecond},
		cwDests:            sync.Map{},
	}
	defer c.Close()

	dest := c.CreateDest("G", "S", -1, util.StandardLogGroupClass, nil).(*cwDest)
	require.Nil(t, c.retryHeap, "no retry heap expected at concurrency 0")

	require.NoError(t, dest.Publish([]logs.LogEvent{
		&retryHeapTestEvent{msg: emfMessage, ts: time.Now()},
	}))
	require.True(t, dest.isEMF)

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) && len(rec.attempts()) < 2 {
		time.Sleep(25 * time.Millisecond)
	}

	got := rec.attempts()
	require.GreaterOrEqual(t, len(got), 2, "expected an in-place retry; got %d attempts", len(got))
	for i, h := range got {
		require.Equal(t, "json/emf", h, "attempt %d lost the EMF header in synchronous mode", i+1)
	}
}
