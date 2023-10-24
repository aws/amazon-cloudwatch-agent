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
	handlerID   = "cloudwatchagent.ClientStatsHandler"
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
	mu sync.Mutex

	filter           agent.OperationsFilter
	getOperationName func(ctx context.Context) string
	getRequestID     func(ctx context.Context) string

	statsByOperation map[string]agent.Stats
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
		statsByOperation: make(map[string]agent.Stats),
	}
}

func (csh *clientStatsHandler) ID() string {
	return handlerID
}

func (csh *clientStatsHandler) Position() awsmiddleware.HandlerPosition {
	return awsmiddleware.Before
}

func (csh *clientStatsHandler) HandleRequest(ctx context.Context, r *http.Request) {
	operation := csh.getOperationName(ctx)
	if !csh.filter.IsAllowed(operation) {
		return
	}
	csh.mu.Lock()
	defer csh.mu.Unlock()
	requestID := csh.getRequestID(ctx)
	recorder := &requestRecorder{start: time.Now()}
	recorder.payloadBytes, _ = io.Copy(io.Discard, r.Body)
	csh.requestCache.Set(requestID, recorder, ttlcache.DefaultTTL)
}

func (csh *clientStatsHandler) HandleResponse(ctx context.Context, r *http.Response) {
	operation := csh.getOperationName(ctx)
	if !csh.filter.IsAllowed(operation) {
		return
	}
	csh.mu.Lock()
	defer csh.mu.Unlock()
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
	latency := time.Since(recorder.start).Milliseconds()
	stats.LatencyMillis = aws.Int64(latency)
	csh.statsByOperation[operation] = stats
}

func (csh *clientStatsHandler) Stats(operation string) agent.Stats {
	csh.mu.Lock()
	defer csh.mu.Unlock()
	stats := csh.statsByOperation[operation]
	csh.statsByOperation[operation] = agent.Stats{}
	return stats
}
