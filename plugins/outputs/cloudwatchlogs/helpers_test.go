// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudwatchlogs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/influxdata/telegraf/testutil"

	"github.com/aws/amazon-cloudwatch-agent/internal"
)

// countingEvent records Done() so undelivered events cannot be silently acknowledged.
type countingEvent struct {
	msg  string
	ts   time.Time
	done *atomic.Int32
}

func (e *countingEvent) Message() string { return e.msg }

func (e *countingEvent) Time() time.Time { return e.ts }

func (e *countingEvent) Done() {
	if e.done != nil {
		e.done.Add(1)
	}
}

// cwlServer serves the CWL JSON1.1 protocol. putErr selects the PutLogEvents outcome.
func cwlServer(putCalls *atomic.Int32, putErr string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := r.Header.Get("X-Amz-Target")
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		switch {
		case strings.Contains(target, "PutLogEvents"):
			putCalls.Add(1)
			if putErr != "" {
				w.Header().Set("X-Amzn-Errortype", putErr)
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"__type":"` + putErr + `","message":"injected"}`))
				return
			}
			_, _ = w.Write([]byte(`{}`))
		case strings.Contains(target, "DescribeLogGroups"):
			_, _ = w.Write([]byte(`{"logGroups":[]}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
}

func newPlugin(endpoint string, concurrency int) *CloudWatchLogs {
	return &CloudWatchLogs{
		Log:                testutil.Logger{Name: "test"},
		Region:             "us-east-1",
		AccessKey:          "access_key",
		SecretKey:          "secret_key",
		EndpointOverride:   endpoint,
		Concurrency:        concurrency,
		ForceFlushInterval: internal.Duration{Duration: 100 * time.Millisecond},
		cwDests:            sync.Map{},
	}
}

type retryHeapTestEvent struct {
	msg string
	ts  time.Time
}

func (e *retryHeapTestEvent) Message() string { return e.msg }

func (e *retryHeapTestEvent) Time() time.Time { return e.ts }

func (e *retryHeapTestEvent) Done() {}
