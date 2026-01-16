// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"container/heap"
	"errors"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
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
	heap      retryHeapImpl
	mutex     sync.RWMutex
	semaphore chan struct{} // Size enforcer
	stopCh    chan struct{}
	maxSize   int
}

var _ RetryHeap = (*retryHeap)(nil)

// NewRetryHeap creates a new retry heap with the specified maximum size
func NewRetryHeap(maxSize int) RetryHeap {
	rh := &retryHeap{
		heap:      make(retryHeapImpl, 0, maxSize),
		maxSize:   maxSize,
		semaphore: make(chan struct{}, maxSize), // Semaphore for size enforcement
		stopCh:    make(chan struct{}),
	}
	heap.Init(&rh.heap)
	return rh
}

// Push adds a batch to the heap, blocking if full
func (rh *retryHeap) Push(batch *logEventBatch) error {
	// Acquire semaphore slot (blocks if at maxSize capacity)
	select {
	case rh.semaphore <- struct{}{}:
		// add batch to heap with mutex protection
		rh.mutex.Lock()
		heap.Push(&rh.heap, batch)
		rh.mutex.Unlock()
		return nil
	case <-rh.stopCh:
		return errors.New("retry heap stopped")
	}
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
		// Release semaphore slot for each popped batch
		<-rh.semaphore
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
	close(rh.stopCh)
}

// RetryHeapProcessor manages the retry heap and moves ready batches back to sender queue
type RetryHeapProcessor struct {
	retryHeap        RetryHeap
	senderPool       Sender
	stopCh           chan struct{}
	logger           telegraf.Logger
	stopped          bool
	maxRetryDuration time.Duration
}

// NewRetryHeapProcessor creates a new retry heap processor
func NewRetryHeapProcessor(retryHeap RetryHeap, workerPool WorkerPool, service cloudWatchLogsService, targetManager TargetManager, logger telegraf.Logger, maxRetryDuration time.Duration) *RetryHeapProcessor {
	// Create processor's own sender and senderPool
	// Pass retryHeap so failed batches go back to RetryHeap instead of blocking on sync retry
	sender := newSender(logger, service, targetManager, maxRetryDuration, true, retryHeap)
	senderPool := newSenderPool(workerPool, sender)

	return &RetryHeapProcessor{
		retryHeap:        retryHeap,
		senderPool:       senderPool,
		stopCh:           make(chan struct{}),
		logger:           logger,
		stopped:          false,
		maxRetryDuration: maxRetryDuration,
	}
}

// Start begins processing the retry heap every 100ms
func (p *RetryHeapProcessor) Start() {
	go p.processLoop()
}

// Stop stops the retry heap processor
func (p *RetryHeapProcessor) Stop() {
	if p.stopped {
		return
	}

	// Process any remaining batches before stopping
	p.processReadyMessages()

	close(p.stopCh)
	p.stopped = true
}

// processLoop runs the main processing loop
func (p *RetryHeapProcessor) processLoop() {
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
	readyBatches := p.retryHeap.PopReady()

	for _, batch := range readyBatches {
		// Check if batch has expired
		if batch.isExpired(p.maxRetryDuration) {
			p.logger.Errorf("Dropping expired batch for %v/%v", batch.Group, batch.Stream)
			batch.updateState()
			continue
		}

		// Submit the batch back to the sender pool (blocks if full)
		p.senderPool.Send(batch)
		p.logger.Debugf("Moved batch from retry heap back to sender pool for %v/%v",
			batch.Group, batch.Stream)
	}
}
