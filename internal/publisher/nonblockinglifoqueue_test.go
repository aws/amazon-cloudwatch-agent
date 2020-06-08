package publisher

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNonBlockingLifoQueue(t *testing.T) {
	queue := NewNonBlockingLifoQueue(2)
	var v interface{}

	queue.Enqueue(1)
	queue.Enqueue(2)
	v, _ = queue.Dequeue()
	assert.Equal(t, 2, v)
	v, _ = queue.Dequeue()
	assert.Equal(t, 1, v)

	queue.Enqueue(1)
	queue.Enqueue(2)
	queue.Enqueue(3)
	v, _ = queue.Dequeue()
	assert.Equal(t, 3, v)
	v, _ = queue.Dequeue()
	assert.Equal(t, 2, v)
	v, _ = queue.Dequeue()
	assert.Equal(t, nil, v)
}
