package publisher

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestBlockingFifoQueue(t *testing.T) {
	queue := NewBlockingFifoQueue(1)
	queue.Enqueue("test")
	result, _ := queue.Dequeue()
	assert.Equal(t, "test", result)
}

func TestBlockingFifoQueue_BlockEnqueue(t *testing.T) {
	start := time.Now()
	queue := NewBlockingFifoQueue(1)
	go func() {
		time.Sleep(time.Second)
		queue.Dequeue()
	}()
	queue.Enqueue("test")
	queue.Enqueue("test")
	assert.True(t, time.Now().Sub(start).Seconds() >= 1.0)
}

func TestBlockingFifoQueue_BlockDequeue(t *testing.T) {
	queue := NewBlockingFifoQueue(1)
	go func() {
		time.Sleep(time.Second)
		queue.Enqueue("test")
	}()
	result, ok := queue.Dequeue()
	assert.Equal(t, nil, result)
	assert.Equal(t, false, ok)
	time.Sleep(2 * time.Second)
	result, ok = queue.Dequeue()
	assert.Equal(t, "test", result)
	assert.Equal(t, true, ok)
}
