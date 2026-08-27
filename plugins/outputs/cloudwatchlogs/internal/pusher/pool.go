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
	Submit(task func())
	Stop()
}

type workerPool struct {
	tasks       chan func()
	workerCount atomic.Int32
	wg          sync.WaitGroup
	stopCh      chan struct{}
	stopLock    sync.RWMutex
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
	for task := range p.tasks {
		runTask(task)
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
func (p *workerPool) Submit(task func()) {
	p.stopLock.RLock()
	defer p.stopLock.RUnlock()
	select {
	case <-p.stopCh:
		return
	default:
		select {
		case p.tasks <- task:
		case <-p.stopCh:
			return
		}
	}
}

// WorkerCount keeps track of the available workers in the pool.
func (p *workerPool) WorkerCount() int32 {
	return p.workerCount.Load()
}

// Stop closes the channels and waits for the workers to stop.
func (p *workerPool) Stop() {
	p.stopLock.Lock()
	defer p.stopLock.Unlock()
	select {
	case <-p.stopCh:
		return
	default:
		close(p.stopCh)
		close(p.tasks)
		p.wg.Wait()
	}
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
	s.workerPool.Submit(func() {
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
}

func (s *senderPool) Stop() {
	// workerpool is stopped by the plugin
	s.sender.Stop()
}
