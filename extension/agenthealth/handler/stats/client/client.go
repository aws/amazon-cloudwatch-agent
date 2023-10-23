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

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
)

const (
	handlerID          = "cloudwatchagent.ClientStatsHandler"
	AllowAllOperations = "*"
)

type Stats interface {
	awsmiddleware.RequestHandler
	awsmiddleware.ResponseHandler
	agent.StatsProvider
}

type StatsConfig struct {
	// Operations is the allowed operation types to gather stats for.
	Operations []string `mapstructure:"operations,omitempty"`
}

type operationRecorder struct {
	lastRequestID string
	start         time.Time
	payloadBytes  int64

	stats agent.Stats
}

type clientStatsHandler struct {
	mu                 sync.Mutex
	allowedOperations  collections.Set[string]
	allowAllOperations bool
	getOperationName   func(ctx context.Context) string
	getRequestID       func(ctx context.Context) string

	operationRecorders map[string]*operationRecorder
}

var _ Stats = (*clientStatsHandler)(nil)

func NewHandler(cfg StatsConfig) Stats {
	allowedOperations := collections.NewSet[string](cfg.Operations...)
	return &clientStatsHandler{
		allowedOperations:  allowedOperations,
		allowAllOperations: allowedOperations.Contains(AllowAllOperations),
		getOperationName:   awsmiddleware.GetOperationName,
		getRequestID:       awsmiddleware.GetRequestID,
		operationRecorders: map[string]*operationRecorder{},
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
	if !csh.allowAllOperations && !csh.allowedOperations.Contains(operation) {
		return
	}
	csh.mu.Lock()
	defer csh.mu.Unlock()
	recorder, ok := csh.operationRecorders[operation]
	if !ok {
		recorder = &operationRecorder{}
	}
	recorder.lastRequestID = csh.getRequestID(ctx)
	recorder.start = time.Now()
	recorder.payloadBytes, _ = io.Copy(io.Discard, r.Body)
	csh.operationRecorders[operation] = recorder
}

func (csh *clientStatsHandler) HandleResponse(ctx context.Context, r *http.Response) {
	operation := csh.getOperationName(ctx)
	if !csh.allowAllOperations && !csh.allowedOperations.Contains(operation) {
		return
	}
	csh.mu.Lock()
	defer csh.mu.Unlock()
	recorder, ok := csh.operationRecorders[operation]
	if !ok {
		return
	}
	recorder.stats = agent.Stats{
		PayloadBytes: aws.Int(int(recorder.payloadBytes)),
		StatusCode:   aws.Int(r.StatusCode),
	}
	if recorder.lastRequestID == csh.getRequestID(ctx) {
		latency := time.Since(recorder.start).Milliseconds()
		recorder.stats.LatencyMillis = aws.Int64(latency)
	}
}

func (csh *clientStatsHandler) Stats(operation string) agent.Stats {
	csh.mu.Lock()
	defer csh.mu.Unlock()
	return csh.operationRecorders[operation].stats
}
