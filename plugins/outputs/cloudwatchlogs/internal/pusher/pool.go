// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"log"
	"sync"
	"sync/atomic"

	"github.com/influxdata/telegraf"
)

type WorkerPool interface {
	// Submit reports whether the task was accepted. False means the pool is shutting down
	// and the task will never run, so the caller must finalize whatever it owns.
	Submit(task func()) bool
	Stop()
}

type workerPool struct {
	tasks       chan func()
	workerCount atomic.Int32
	wg          sync.WaitGroup
	stopCh      chan struct{}
	stopOnce    sync.Once
	stopping    atomic.Bool
}

// NewWorkerPool creates a pool of workers of the specified size.
func NewWorkerPool(size int) WorkerPool {
	p := &workerPool{
		tasks:  make(chan func(), size*2),
		stopCh: make(chan struct{}),
	}
	for i := 0; i < size; i++ {
		p.addWorker()
	}
	return p
}

// addWorker creates and starts a new worker goroutine.
func (p *workerPool) addWorker() {
	p.wg.Add(1)
	p.workerCount.Add(1)
	go p.worker()
}

// worker receives tasks from the channel and executes them.
func (p *workerPool) worker() {
	defer func() {
		p.workerCount.Add(-1)
		p.wg.Done()
	}()
	for {
		select {
		case task := <-p.tasks:
			runTask(task)
		case <-p.stopCh:
			// Drain what is already queued so a shutdown does not silently discard
			// submitted sends, then exit.
			for {
				select {
				case task := <-p.tasks:
					runTask(task)
				default:
					return
				}
			}
		}
	}
}

// runTask isolates a panic to the one task that raised it. Sends run here, so without
// this an unrecovered panic in a worker goroutine terminates the whole agent process.
func runTask(task func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("E! [cloudwatchlogs] Recovered from panic in worker task: %v", r)
		}
	}()
	task()
}

// Submit adds a task to the pool. Blocks until a worker is available to receive the task or the pool is stopped.
//
// Deliberately lock-free: holding a read lock across this blocking send meant Stop's write
// lock could never be acquired, so Stop could not close stopCh to release a Submit parked
// on a saturated pool -- the pool deadlocked on shutdown. Safe without the lock because
// Stop no longer closes p.tasks, only stopCh.
func (p *workerPool) Submit(task func()) bool {
	// Check stopping first: select is random when both cases are ready, so without this a
	// task could be enqueued after Stop and never run, leaving its batch unfinalized.
	if p.stopping.Load() {
		return false
	}
	select {
	case p.tasks <- task:
		return true
	case <-p.stopCh:
		return false
	}
}

// WorkerCount keeps track of the available workers in the pool.
func (p *workerPool) WorkerCount() int32 {
	return p.workerCount.Load()
}

// Stop closes the channels and waits for the workers to stop.
// Stop signals the workers and waits for them to drain and exit. p.tasks is deliberately
// NOT closed: a concurrent Submit would panic on a closed channel, and avoiding the close
// is what lets Submit run lock-free.
//
// This wait is unbounded by design, and it is the one place shutdown genuinely blocks on
// in-flight work: a worker mid-PutLogEvents must be allowed to finish. Callers upstream
// (pusherWaitGroup.Wait) therefore do not bound their own waits either -- doing so only
// relocates the wait here. A worker that never returns (an HTTP call that never times out)
// would stall shutdown; that is bounded in practice by the client's timeouts, not by us.
func (p *workerPool) Stop() {
	p.stopOnce.Do(func() {
		p.stopping.Store(true) // stop accepting before signalling, so Submit answers definitively
		close(p.stopCh)
		p.wg.Wait()
	})
}

// senderPool wraps a Sender with a WorkerPool for concurrent sending.
type senderPool struct {
	workerPool WorkerPool
	sender     Sender
	logger     telegraf.Logger
}

var _ Sender = (*senderPool)(nil)

func newSenderPool(workerPool WorkerPool, sender Sender, logger telegraf.Logger) Sender {
	return &senderPool{
		workerPool: workerPool,
		sender:     sender,
		logger:     logger,
	}
}

// Send submits a send task to the worker pool.
func (s *senderPool) Send(batch *logEventBatch) {
	accepted := s.workerPool.Submit(func() {
		// Recover here, where the batch is in scope, and abandon it. runTask's recover keeps
		// the worker alive but cannot finalize the batch, which would leave the target's
		// breaker halted and its file offsets never advanced. abandon() clears the breaker
		// without persisting offsets, so the events are re-read after restart.
		defer func() {
			if r := recover(); r != nil {
				s.logger.Errorf("Recovered from panic sending batch for %v/%v; abandoning it (will be re-read after restart): %v",
					batch.Group, batch.Stream, r)
				batch.abandon()
			}
		}()
		s.sender.Send(batch)
	})
	if !accepted {
		// The pool is shutting down and will never run this task, so finalize the batch here
		// rather than leaving the target's breaker halted with its offsets never advanced.
		s.logger.Warnf("Worker pool stopped, abandoning batch for %v/%v (will be re-read after restart)",
			batch.Group, batch.Stream)
		batch.abandon()
	}
}

func (s *senderPool) Stop() {
	// workerpool is stopped by the plugin
	s.sender.Stop()
}
