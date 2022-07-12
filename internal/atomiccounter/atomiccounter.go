// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package atomiccounter

import (
	"sync/atomic"
)

type AtomicCounter struct {
	val int64
}

// NewAtomicCounter returns a new counter with the default value of 0.
func NewAtomicCounter() AtomicCounter {
	return AtomicCounter{}
}

func (ac *AtomicCounter) Increment() {
	atomic.AddInt64(&ac.val, 1)
}

func (ac *AtomicCounter) Decrement() {
	atomic.AddInt64(&ac.val, -1)
}

// Get is not safe to use for synchronizing work between goroutines.
// It is just for logging the current value.
func (ac *AtomicCounter) Get() int64 {
	return ac.val
}