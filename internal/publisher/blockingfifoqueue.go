// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package publisher

import (
	"log"
)

// It is a FIFO queue with the functionality that block the caller if the queue size reaches to the maxSize
type BlockingFifoQueue struct {
	queue chan interface{}
}

func NewBlockingFifoQueue(size int) *BlockingFifoQueue {
	if size <= 0 {
		log.Panic("E! Queue Size should be larger than 0!")
	}

	return &BlockingFifoQueue{queue: make(chan interface{}, size)}
}

func (b *BlockingFifoQueue) Enqueue(req interface{}) {
	b.queue <- req
}

func (b *BlockingFifoQueue) Dequeue() (interface{}, bool) {
	select {
	case v := <-b.queue:
		return v, true
	default:
		return nil, false
	}
}
