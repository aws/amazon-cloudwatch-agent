// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package semaphore

import (
	"log"
	"time"
)

type Semaphore interface {
	//Acquire a slot in the semaphore with a timeout
	Acquire(timeout time.Duration) bool

	//Release a slot in the semaphore.
	Release()

	//GetLimit returns semaphore limit.
	GetLimit() int

	//GetCount returns current number of occupied slots in semaphore.
	GetCount() int

	//Done closes down the semaphore
	Done()
}

type semaphore struct {
	slots chan struct{}
}

func NewSemaphore(limit int) Semaphore {
	if limit < 0 {
		log.Panic("E! Semaphore size limit must not be negative")
	}

	sem := &semaphore{
		slots: make(chan struct{}, limit),
	}

	for i := 0; i < cap(sem.slots); i++ {
		sem.slots <- struct{}{}
	}

	return sem
}

func (sem *semaphore) Acquire(timeout time.Duration) bool {
	if (timeout > 0) {
		defer timer.Stop()
		select {
		case <-sem.slots:
			return true
		case <-time.After(timeout):
		}
	}

	select {
	case <-sem.slots:
		return true
	default:
		return false
	}
}

// Release the acquired semaphore.
func (sem *semaphore) Release() {
	select {
	case sem.slots <- struct{}{}:
	default:
		log.Printf("E! Semaphore released more than held")
	}
}

func (sem *semaphore) GetCount() int {
	return cap(sem.slots) - len(sem.slots)
}

func (sem *semaphore) GetLimit() int {
	return cap(sem.slots)
}

func (sem *semaphore) Done() {
	close(sem.slots)
}
