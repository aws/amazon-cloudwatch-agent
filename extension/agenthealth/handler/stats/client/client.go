// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/jellydator/ttlcache/v3"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

const (
	handlerID   = "cloudwatchagent.ClientStats"
	ttlDuration = 10 * time.Second
	cacheSize   = 1000
)

type Stats interface {
	awsmiddleware.RequestHandler
	awsmiddleware.ResponseHandler
	agent.StatsProvider
}

type requestRecorder struct {
	start        time.Time
	payloadBytes int64
}

type clientStatsHandler struct {
	filter           agent.OperationsFilter
	getOperationName func(ctx context.Context) string
	getRequestID     func(ctx context.Context) string

	statsByOperation sync.Map
	requestCache     *ttlcache.Cache[string, *requestRecorder]
}

var _ Stats = (*clientStatsHandler)(nil)

func NewHandler(filter agent.OperationsFilter) Stats {
	requestCache := ttlcache.New[string, *requestRecorder](
		ttlcache.WithTTL[string, *requestRecorder](ttlDuration),
		ttlcache.WithCapacity[string, *requestRecorder](cacheSize),
		ttlcache.WithDisableTouchOnHit[string, *requestRecorder](),
	)
	go requestCache.Start()
	return &clientStatsHandler{
		filter:           filter,
		getOperationName: awsmiddleware.GetOperationName,
		getRequestID:     awsmiddleware.GetRequestID,
		requestCache:     requestCache,
	}
}

func (csh *clientStatsHandler) ID() string {
	return handlerID
}

func (csh *clientStatsHandler) Position() awsmiddleware.HandlerPosition {
	return awsmiddleware.After
}

func (csh *clientStatsHandler) HandleRequest(ctx context.Context, r *http.Request) {
	operation := csh.getOperationName(ctx)
	if !csh.filter.IsAllowed(operation) {
		return
	}
	requestID := csh.getRequestID(ctx)
	recorder := &requestRecorder{start: time.Now()}
	if r.ContentLength > 0 {
		recorder.payloadBytes = r.ContentLength
	} else if r.Body != nil {
		rsc, ok := r.Body.(aws.ReaderSeekerCloser)
		if !ok {
			rsc = aws.ReadSeekCloser(r.Body)
		}
		if length, _ := aws.SeekerLen(rsc); length > 0 {
			recorder.payloadBytes = length
		} else if body, err := r.GetBody(); err == nil {
			recorder.payloadBytes, _ = io.Copy(io.Discard, body)
		}
	}
	csh.requestCache.Set(requestID, recorder, ttlcache.DefaultTTL)
}

func (csh *clientStatsHandler) HandleResponse(ctx context.Context, r *http.Response) {
	operation := csh.getOperationName(ctx)
	if !csh.filter.IsAllowed(operation) {
		return
	}
	requestID := csh.getRequestID(ctx)
	item, ok := csh.requestCache.GetAndDelete(requestID)
	if !ok {
		return
	}
	recorder := item.Value()
	stats := agent.Stats{
		PayloadBytes: aws.Int(int(recorder.payloadBytes)),
		StatusCode:   aws.Int(r.StatusCode),
	}
	latency := time.Since(recorder.start)
	stats.LatencyMillis = aws.Int64(latency.Milliseconds())
	csh.statsByOperation.Store(operation, stats)
}

func (csh *clientStatsHandler) Stats(operation string) agent.Stats {
	value, ok := csh.statsByOperation.Load(operation)
	if !ok {
		return agent.Stats{}
	}
	stats, ok := value.(agent.Stats)
	if !ok {
		return agent.Stats{}
	}
	return stats
}
