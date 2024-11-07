// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"io"
	"log"
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

	//Map to track status codes by API
	StatusCodesByAPI map[string][]int // e.g., [200, 400, 404, ...]
	resetTimer       *time.Ticker
	stopTimer        chan struct{}
}

var _ Stats = (*clientStatsHandler)(nil)

func NewHandler(filter agent.OperationsFilter) Stats {
	requestCache := ttlcache.New[string, *requestRecorder](
		ttlcache.WithTTL[string, *requestRecorder](ttlDuration),
		ttlcache.WithCapacity[string, *requestRecorder](cacheSize),
		ttlcache.WithDisableTouchOnHit[string, *requestRecorder](),
	)
	go requestCache.Start()
	csh := &clientStatsHandler{
		filter:           filter,
		getOperationName: awsmiddleware.GetOperationName,
		getRequestID:     awsmiddleware.GetRequestID,
		requestCache:     requestCache,
		StatusCodesByAPI: make(map[string][]int),
		stopTimer:        make(chan struct{}),
	}
	csh.startResetTimer()
	return csh
}

func (csh *clientStatsHandler) startResetTimer() {
	csh.resetTimer = time.NewTicker(5 * time.Minute)

	go func() {
		for {
			select {
			case <-csh.resetTimer.C:
				csh.resetStatusCodes()
			case <-csh.stopTimer:
				csh.resetTimer.Stop()
				return
			}
		}
	}()
}

func (csh *clientStatsHandler) resetStatusCodes() {
	log.Println("Resetting status codes counts for all APIs")
	for api := range csh.StatusCodesByAPI {
		// Reset the count for each status code for the API
		csh.StatusCodesByAPI[api] = make([]int, 10) // Adjust size based on expected status codes
	}
}

// Stop the reset timer when no longer needed
func (csh *clientStatsHandler) Stop() {
	close(csh.stopTimer)
}

func (csh *clientStatsHandler) ID() string {
	return handlerID
}

func (csh *clientStatsHandler) Position() awsmiddleware.HandlerPosition {
	return awsmiddleware.After
}

func (csh *clientStatsHandler) HandleRequest(ctx context.Context, r *http.Request) {
	operation := csh.getOperationName(ctx)
	log.Printf("Handling request for operation: %s", operation)

	if !csh.filter.IsAllowed(operation) {
		log.Printf("Operation %s is not allowed by the filter, skipping request handling.", operation)
		return
	}

	requestID := csh.getRequestID(ctx)
	log.Printf("Generated request ID: %s", requestID)

	recorder := &requestRecorder{start: time.Now()}

	if r.ContentLength > 0 {
		recorder.payloadBytes = r.ContentLength
		log.Printf("Request content length: %d bytes", r.ContentLength)
	} else if r.Body != nil {
		rsc, ok := r.Body.(aws.ReaderSeekerCloser)
		if !ok {
			rsc = aws.ReadSeekCloser(r.Body)
		}
		if length, _ := aws.SeekerLen(rsc); length > 0 {
			recorder.payloadBytes = length
			log.Printf("Seeker length of request body: %d bytes", length)
		} else if body, err := r.GetBody(); err == nil {
			recorder.payloadBytes, _ = io.Copy(io.Discard, body)
			log.Printf("Calculated body length by copying: %d bytes", recorder.payloadBytes)
		}
	}

	log.Printf("Storing recorder in cache for request ID: %s", requestID)
	csh.requestCache.Set(requestID, recorder, ttlcache.DefaultTTL)
}

func (csh *clientStatsHandler) HandleResponse(ctx context.Context, r *http.Response) {
	operation := csh.getOperationName(ctx)
	log.Printf("Handling response for operation: %s", operation)

	if !csh.filter.IsAllowed(operation) {
		log.Printf("Operation %s is not allowed by the filter, skipping response handling.", operation)
		return
	}

	requestID := csh.getRequestID(ctx)
	log.Printf("Retrieved request ID for response: %s", requestID)

	item, ok := csh.requestCache.GetAndDelete(requestID)
	if !ok {
		log.Printf("No request recorder found in cache for request ID: %s", requestID)
		return
	}

	recorder := item.Value()
	stats := agent.Stats{
		PayloadBytes: aws.Int(int(recorder.payloadBytes)),
		StatusCode:   aws.Int(r.StatusCode),
	}

	latency := time.Since(recorder.start)
	stats.LatencyMillis = aws.Int64(latency.Milliseconds())

	log.Printf("Request stats for operation %s: PayloadBytes=%d, StatusCode=%d, LatencyMillis=%d",
		operation, recorder.payloadBytes, r.StatusCode, stats.LatencyMillis)

	csh.UpdateStatusCode(operation, r.StatusCode)

	csh.statsByOperation.Store(operation, stats)
	log.Printf("Stored stats for operation: %s", operation)
}

func (csh *clientStatsHandler) UpdateStatusCode(api string, statusCode int) {
	if csh.StatusCodesByAPI == nil {
		csh.StatusCodesByAPI = make(map[string][]int)
		log.Printf("Initialized StatusCodesByAPI map")
	}

	if _, exists := csh.StatusCodesByAPI[api]; !exists {
		csh.StatusCodesByAPI[api] = make([]int, 10) // Adjust size based on expected status codes
		log.Printf("Initialized status code count for API: %s", api)
	}

	switch statusCode {
	case 200:
		csh.StatusCodesByAPI[api][0]++
		log.Printf("Incremented status code 200 count for API: %s, new count: %d", api, csh.StatusCodesByAPI[api][0])
	case 400:
		csh.StatusCodesByAPI[api][1]++
		log.Printf("Incremented status code 400 count for API: %s, new count: %d", api, csh.StatusCodesByAPI[api][1])
	case 404:
		csh.StatusCodesByAPI[api][2]++
		log.Printf("Incremented status code 404 count for API: %s, new count: %d", api, csh.StatusCodesByAPI[api][2])
	// Add additional cases for other status codes as necessary
	default:
		log.Printf("Received untracked status code %d for API: %s", statusCode, api)
	}
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
