// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package semaphore

import (
	"time"
	"sync"
	"testing"
	"github.com/stretchr/testify/assert"
)

func checkLimitAndCount(t *testing.T, sem Semaphore, expectedLimit, expectedCount int) {
	limit := sem.GetLimit()
	assert.Equal(t, expectedLimit, limit)
	count := sem.GetCount()
	assert.Equal(t, expectedCount, count)
}

func TestNewSemaphore_NegativeLimit(t *testing.T) {
	assert.Panics(t, func() { _ = NewSemaphore(-1) })
}

func TestSemaphore_AcquireReleaseSimple(t *testing.T) {
	sem := NewSemaphore(2)
	checkLimitAndCount(t, sem, 2, 0)

	ok := sem.Acquire(0)
	assert.True(t, ok)
	checkLimitAndCount(t, sem, 2, 1)

	ok = sem.Acquire(time.Second)
	assert.True(t, ok)
	checkLimitAndCount(t, sem, 2, 2)

	sem.Release()
	checkLimitAndCount(t, sem, 2, 1)

	sem.Release()
	checkLimitAndCount(t, sem, 2, 0)
}

func TestSemaphore_ReleaseWithZeroCapacity(t *testing.T) {
	sem := NewSemaphore(0)
	sem.Release()
	checkLimitAndCount(t, sem, 0, 0)
	ok := sem.Acquire(0)
	assert.False(t, ok)
	checkLimitAndCount(t, sem, 0, 0)
}

func TestSemaphore_ReleaseMoreThanAcquire(t *testing.T) {
	sem := NewSemaphore(1)
	ok := sem.Acquire(0)
	assert.True(t, ok)
	checkLimitAndCount(t, sem, 1, 1)
	sem.Release()
	checkLimitAndCount(t, sem, 1, 0)
	sem.Release()
	checkLimitAndCount(t, sem, 1, 0)
}

func TestSemaphore_AcquireRelease(t *testing.T) {
	cases := []struct {
		testName      string
		limit         int
		numberOfFiles int
	}{
		{
			testName:    "Acquire release under limit",
			limit: 100,
			numberOfFiles: 10,
		},
		{
			testName:    "Acquire release over limit ",
			limit: 10,
			numberOfFiles: 100,
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			sem := NewSemaphore(c.limit)

			channel := make(chan struct{})
			wg := sync.WaitGroup{}
			for i := 0; i < c.numberOfFiles; i++ {
				wg.Add(1)
				go func() {
					<-channel
					ok := sem.Acquire(time.Second)
					assert.True(t, ok)
					sem.Release()
					wg.Done()
				}()
			}

			close(channel) // start
			wg.Wait()

			checkLimitAndCount(t, sem, c.limit, 0)
		})
	}
}

