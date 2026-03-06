// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"container/heap"
	"errors"
	"sync"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/aws/amazon-cloudwatch-agent/internal/retryer"
)

// retryHeapImpl implements heap.Interface for logEventBatch sorted by nextRetryTime
type retryHeapImpl []*logEventBatch

var _ heap.Interface = (*retryHeapImpl)(nil)

func (h retryHeapImpl) Len() int { return len(h) }

func (h retryHeapImpl) Less(i, j int) bool {
	return h[i].nextRetryTime.Before(h[j].nextRetryTime)
}

func (h retryHeapImpl) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *retryHeapImpl) Push(x interface{}) {
	*h = append(*h, x.(*logEventBatch))
}

func (h *retryHeapImpl) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // don't stop the GC from reclaiming the item eventually
	*h = old[0 : n-1]
	return item
}

// RetryHeap manages failed batches during their retry wait periods
type RetryHeap interface {
	Push(batch *logEventBatch) error
	PopReady() []*logEventBatch
	Size() int
	Stop()
}

type retryHeap struct {
	heap    retryHeapImpl
	mutex   sync.RWMutex
	stopCh  chan struct{}
	stopped bool
	logger  telegraf.Logger
}

var _ RetryHeap = (*retryHeap)(nil)

// NewRetryHeap creates a new retry heap (unbounded)
func NewRetryHeap(logger telegraf.Logger) RetryHeap {
	rh := &retryHeap{
		heap:   make(retryHeapImpl, 0),
		stopCh: make(chan struct{}),
		logger: logger,
	}
	heap.Init(&rh.heap)
	return rh
}

// Push adds a batch to the heap (non-blocking)
func (rh *retryHeap) Push(batch *logEventBatch) error {
	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	if rh.stopped {
		return errors.New("retry heap stopped")
	}

	heap.Push(&rh.heap, batch)
	return nil
}

// PopReady returns all batches that are ready for retry (nextRetryTime <= now)
func (rh *retryHeap) PopReady() []*logEventBatch {
	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	now := time.Now()
	var ready []*logEventBatch

	// Pop all batches that are ready for retry
	for len(rh.heap) > 0 && !rh.heap[0].nextRetryTime.After(now) {
		batch := heap.Pop(&rh.heap).(*logEventBatch)
		ready = append(ready, batch)
	}

	return ready
}

// Size returns the current number of batches in the heap
func (rh *retryHeap) Size() int {
	rh.mutex.RLock()
	defer rh.mutex.RUnlock()
	return len(rh.heap)
}

// Stop stops the retry heap
func (rh *retryHeap) Stop() {
	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	if rh.stopped {
		return
	}
	close(rh.stopCh)
	rh.stopped = true
}

// RetryHeapProcessor manages the retry heap and moves ready batches back to sender queue
type RetryHeapProcessor struct {
	retryHeap  RetryHeap
	senderPool Sender
	retryer    *retryer.LogThrottleRetryer
	stopCh     chan struct{}
	logger     telegraf.Logger
	stopped    bool
	stopMu     sync.Mutex
	wg         sync.WaitGroup
}

// NewRetryHeapProcessor creates a new retry heap processor
func NewRetryHeapProcessor(retryHeap RetryHeap, workerPool WorkerPool, service cloudWatchLogsService, targetManager TargetManager, logger telegraf.Logger, retryer *retryer.LogThrottleRetryer) *RetryHeapProcessor {
	// Create processor's own sender and senderPool
	// Pass retryHeap so failed batches go back to RetryHeap instead of blocking on sync retry
	sender := newSender(logger, service, targetManager, retryHeap)
	senderPool := newSenderPool(workerPool, sender)

	return &RetryHeapProcessor{
		retryHeap:  retryHeap,
		senderPool: senderPool,
		retryer:    retryer,
		stopCh:     make(chan struct{}),
		logger:     logger,
		stopped:    false,
	}
}

// Start begins processing the retry heap every 100ms
func (p *RetryHeapProcessor) Start() {
	p.wg.Add(1)
	go p.processLoop()
}

// Stop stops the retry heap processor
func (p *RetryHeapProcessor) Stop() {
	p.stopMu.Lock()
	defer p.stopMu.Unlock()

	if p.stopped {
		return
	}

	// Flush remaining ready batches before marking as stopped
	p.flushReadyBatches()

	p.stopped = true

	if p.retryer != nil {
		p.retryer.Stop()
	}
	p.senderPool.Stop()
	close(p.stopCh)
	p.wg.Wait()
}

// processLoop runs the main processing loop
func (p *RetryHeapProcessor) processLoop() {
	defer p.wg.Done()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.processReadyMessages()
		case <-p.stopCh:
			return
		}
	}
}

// processReadyMessages checks the heap for ready batches and moves them back to sender queue
func (p *RetryHeapProcessor) processReadyMessages() {
	p.stopMu.Lock()
	if p.stopped {
		p.stopMu.Unlock()
		return
	}
	p.stopMu.Unlock()

	p.flushReadyBatches()
}

// flushReadyBatches pops ready batches from the heap and sends them.
// Called by both processReadyMessages and Stop.
func (p *RetryHeapProcessor) flushReadyBatches() {
	readyBatches := p.retryHeap.PopReady()

	for _, batch := range readyBatches {
		// Check if batch has expired
		if batch.isExpired() {
			p.logger.Errorf("Dropping expired batch for %v/%v", batch.Group, batch.Stream)
			batch.done() // Resume circuit breaker to allow target to process new batches
			continue
		}

		// Submit the batch back to the sender pool (blocks if full)
		p.senderPool.Send(batch)
		p.logger.Debugf("Moved batch from retry heap back to sender pool for %v/%v",
			batch.Group, batch.Stream)
	}
}
