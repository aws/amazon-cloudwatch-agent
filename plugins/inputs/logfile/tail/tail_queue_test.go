// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tail

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTailerEnqueueAndDequeue(t *testing.T) {
	tailerEnqueue := NewTailerFifoQueue()
	tailerEnqueue.Enqueue()
	tailerEnqueue.Enqueue()
	assert.Equal(t, 2, tailerEnqueue.Size())

	tailerEnqueue.Dequeue()
	assert.Equal(t, 1, tailerEnqueue.Size())

	tailerEnqueue.Dequeue()
	assert.Equal(t, 0, tailerEnqueue.Size())
}
