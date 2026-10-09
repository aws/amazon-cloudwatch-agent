// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"container/heap"
	"errors"
	"sync"
	"sync/atomic"
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

// stopFlushTimeout bounds the shutdown flush so a saturated worker pool cannot hang Stop.
const stopFlushTimeout = 5 * time.Second

// stopWaitTimeout bounds the wait for the process loop, which may be parked in Submit.
const stopWaitTimeout = 5 * time.Second

// waitTimeout reports whether wg finished within d.
func waitTimeout(wg *sync.WaitGroup, d time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

// RetryHeap manages failed batches during their retry wait periods
type RetryHeap interface {
	Push(batch *logEventBatch) error
	PopReady() []*logEventBatch
	Size() int
	Close()
}

type retryHeap struct {
	heap    retryHeapImpl
	mutex   sync.Mutex
	stopped bool
	logger  telegraf.Logger
}

var _ RetryHeap = (*retryHeap)(nil)

// NewRetryHeap creates a new retry heap (unbounded)
func NewRetryHeap(logger telegraf.Logger) RetryHeap {
	rh := &retryHeap{
		heap:   make(retryHeapImpl, 0),
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
	rh.mutex.Lock()
	defer rh.mutex.Unlock()
	return len(rh.heap)
}

// Close releases the retry heap and rejects further pushes. There is no background
// goroutine to shut down; this only flips the stopped flag.
func (rh *retryHeap) Close() {
	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	if rh.stopped {
		return
	}
	rh.stopped = true
}

// RetryHeapProcessor manages the retry heap and moves ready batches back to sender queue
type RetryHeapProcessor struct {
	retryHeap  RetryHeap
	senderPool Sender
	stopCh     chan struct{}
	logger     telegraf.Logger
	stopped    atomic.Bool
	stopOnce   sync.Once
	wg         sync.WaitGroup
}

// NewRetryHeapProcessor creates a new retry heap processor
func NewRetryHeapProcessor(retryHeap RetryHeap, workerPool WorkerPool, service cloudWatchLogsService, targetManager TargetManager, logger telegraf.Logger) *RetryHeapProcessor {
	// Create processor's own sender and senderPool
	// Pass retryHeap so failed batches go back to RetryHeap instead of blocking on sync retry
	sender := newSender(logger, service, targetManager, retryHeap)
	senderPool := newSenderPool(workerPool, sender, logger)

	return &RetryHeapProcessor{
		retryHeap:  retryHeap,
		senderPool: senderPool,
		stopCh:     make(chan struct{}),
		logger:     logger,
	}
}

// Start begins processing the retry heap every 100ms
func (p *RetryHeapProcessor) Start() {
	p.wg.Add(1)
	go p.processLoop()
}

// Stop stops the retry heap processor
func (p *RetryHeapProcessor) Stop() {
	p.stopOnce.Do(func() {
		// Best-effort flush of remaining ready batches, but bounded: the flush submits into
		// the worker pool, which blocks while the pool is saturated. The pool is stopped
		// after this processor, and Submit falls through on the pool's stopCh, so a flush
		// still in flight unblocks there instead of hanging shutdown here.
		flushed := make(chan struct{})
		go func() {
			defer close(flushed)
			p.flushReadyBatches()
		}()
		select {
		case <-flushed:
		case <-time.After(stopFlushTimeout):
			p.logger.Warnf("Retry-heap flush did not finish within %v; continuing shutdown "+
				"(unflushed batches are re-read after restart)", stopFlushTimeout)
		}

		// Release the process loop BEFORE waiting on it. Holding a lock that
		// processReadyMessages also takes across wg.Wait() deadlocks: a tick landing in
		// that window parks the loop on the lock, so it never observes stopCh.
		p.stopped.Store(true)
		close(p.stopCh)

		// Bounded too: processLoop may already be parked in workerPool.Submit on a saturated
		// pool, and the plugin stops that pool only AFTER this returns, so an unbounded wait
		// deadlocks. Returning lets Close reach workerPool.Stop, which releases the Submit;
		// the loop then observes stopCh and exits.
		if !waitTimeout(&p.wg, stopWaitTimeout) {
			p.logger.Warnf("Retry-heap process loop still running after %v; continuing shutdown "+
				"(it exits once the worker pool is stopped)", stopWaitTimeout)
		}

		// Downstream stops only once the producer loop is confirmed dead.
		p.senderPool.Stop()
	})
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
	// A panic here would unwind processLoop via its deferred wg.Done() and permanently
	// strand the retry heap, halting retries for every target. Recover so one bad batch
	// cannot stop retries for all targets; the next tick continues normally.
	defer func() {
		if r := recover(); r != nil {
			p.logger.Errorf("Recovered from panic while processing retry heap: %v", r)
		}
	}()

	if p.stopped.Load() {
		return
	}

	p.flushReadyBatches()
}

// flushReadyBatches pops ready batches from the heap and sends them.
// Called by both processReadyMessages and Stop.
func (p *RetryHeapProcessor) flushReadyBatches() {
	for _, batch := range p.retryHeap.PopReady() {
		p.flushBatch(batch)
	}
}

// flushBatch finalizes or resubmits a single batch. Recovery is scoped per batch because
// PopReady has already removed every ready batch from the heap: a panic unwinding the whole
// loop would silently lose the batches queued behind the failing one, and Stop's flush has
// no outer recover at all.
func (p *RetryHeapProcessor) flushBatch(batch *logEventBatch) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Errorf("Recovered from panic while processing retry batch for %v/%v: %v",
				batch.Group, batch.Stream, r)
		}
	}()

	// Check if batch has expired
	if batch.isExpired() {
		p.logger.Errorf("Dropping expired batch for %v/%v", batch.Group, batch.Stream)
		// Permanent give-up: persist state and clear the breaker, but do not
		// report these events as delivered.
		batch.drop()
		return
	}

	// Submit the batch back to the sender pool (blocks if full)
	p.senderPool.Send(batch)
	p.logger.Debugf("Moved batch from retry heap back to sender pool for %v/%v",
		batch.Group, batch.Stream)
}
