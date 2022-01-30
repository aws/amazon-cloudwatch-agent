// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package publisher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNonBlockingFifoQueue(t *testing.T) {
	queue := NewNonBlockingFifoQueue(2)
	var v interface{}
	var ok bool

	queue.Enqueue(1)
	queue.Enqueue(2)
	v, ok = queue.Dequeue()
	assert.Equal(t, 1, v)
	assert.Equal(t, true, ok)
	v, ok = queue.Dequeue()
	assert.Equal(t, 2, v)
	assert.Equal(t, true, ok)

	queue.Enqueue(1)
	queue.Enqueue(2)
	queue.Enqueue(3)
	v, ok = queue.Dequeue()
	assert.Equal(t, 2, v)
	assert.Equal(t, true, ok)
	v, ok = queue.Dequeue()
	assert.Equal(t, 3, v)
	assert.Equal(t, true, ok)
	v, ok = queue.Dequeue()
	assert.Equal(t, nil, v)
	assert.Equal(t, false, ok)
}
