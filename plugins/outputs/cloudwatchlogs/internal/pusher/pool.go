// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package pusher

import (
	"sync"
	"sync/atomic"
	"time"
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
		task()
	}
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
}

var _ Sender = (*senderPool)(nil)

func newSenderPool(workerPool WorkerPool, sender Sender) Sender {
	return &senderPool{
		workerPool: workerPool,
		sender:     sender,
	}
}

// Send submits a send task to the worker pool.
func (s *senderPool) Send(batch *logEventBatch) {
	s.workerPool.Submit(func() {
		s.sender.Send(batch)
	})
}

func (s *senderPool) Stop() {
	// workerpool is stopped by the plugin
}

// SetRetryDuration sets the retry duration on the wrapped Sender.
func (s *senderPool) SetRetryDuration(duration time.Duration) {
	s.sender.SetRetryDuration(duration)
}

// RetryDuration returns the retry duration of the wrapped Sender.
func (s *senderPool) RetryDuration() time.Duration {
	return s.sender.RetryDuration()
}
