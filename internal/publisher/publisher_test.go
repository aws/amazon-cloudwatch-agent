// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package publisher

import (
	"io"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// testClient will register its "publish" method to publisher
type testClient struct {
	result []string
	sync.Mutex
}

func (c *testClient) publish(req interface{}) {
	c.Lock()
	defer c.Unlock()
	r := req.(string)
	c.result = append(c.result, r)
}

func (c *testClient) publishWith1sLatency(req interface{}) {
	c.publish(req)
	time.Sleep(1 * time.Second)
}

func (c *testClient) publishWith5sLatency(req interface{}) {
	c.publish(req)
	time.Sleep(5 * time.Second)
}

func (c *testClient) getResult() []string {
	c.Lock()
	defer c.Unlock()
	return c.result
}

func TestPublisher_PublishWithNonBlockFifoQueue(t *testing.T) {
	c := &testClient{}
	publisher, _ := NewPublisher(NewNonBlockingFifoQueue(2), 1, 2*time.Second, c.publish)
	publisher.Publish("req1")
	publisher.Publish("req2")
	publisher.Close()
	assert.Equal(t, []string{"req1", "req2"}, c.getResult())
}

func TestPublisher_PublishWithNonBlockFifoQueueSleep(t *testing.T) {
	c := &testClient{}
	publisher, _ := NewPublisher(NewNonBlockingFifoQueue(2), 1, 2*time.Second, c.publish)
	publisher.Publish("req1")
	time.Sleep(100 * time.Millisecond)
	publisher.Publish("req2")
	publisher.Close()
	assert.Equal(t, []string{"req1", "req2"}, c.getResult())
}

func TestPublisher_DrainTimeout(t *testing.T) {
	start := time.Now()
	c := &testClient{}
	publisher, _ := NewPublisher(NewNonBlockingFifoQueue(2), 1, 2*time.Second, c.publishWith5sLatency)
	publisher.Publish("req1")
	publisher.Publish("req2")
	publisher.Close()
	// drain queue timeout 2s + on fly request timeout 1s = 3s (expected)
	assert.True(t, time.Since(start) < 4*time.Second)
}

// testClientNoMutex is to test whether need memory barrier when concurrency of publisher is 1
type testClientNoMutex struct {
	counter int32
}

func (c *testClientNoMutex) publish(req interface{}) {
	r := req.(int)
	atomic.AddInt32(&c.counter, int32(r))
}

func TestPublisher_ClientNoMutex(t *testing.T) {
	log.SetOutput(io.Discard)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
	})
	publishRequestNum := 100000
	c := &testClientNoMutex{}
	publisher, _ := NewPublisher(NewNonBlockingFifoQueue(10), 1, 2*time.Second, c.publish)
	for i := 0; i < publishRequestNum; i++ {
		publisher.Publish(1)
	}
	publisher.Close()
	expectNonZeroDroppedRequest := 100
	assert.Lessf(t, c.counter, int32(publishRequestNum-expectNonZeroDroppedRequest),
		"Less than publish requests actually published due to dropped requests")
}

// testClientLongDelay is to test publisher latency with nonBlockingQueue
type testClientLongDelay struct {
	counter int32
}

func (c *testClientLongDelay) publish(req interface{}) {
	r := req.(int)
	atomic.AddInt32(&c.counter, int32(r))
	time.Sleep(time.Minute)
}
func TestPublisher_ClientLongDelay(t *testing.T) {
	log.SetOutput(io.Discard)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
	})
	c := &testClientLongDelay{}
	publisher, _ := NewPublisher(NewNonBlockingFifoQueue(20), 10, 5*time.Second, c.publish)
	start := time.Now()
	for i := 0; i < 30; i++ {
		publisher.Publish(1)
	}
	// Send 30 requests to the publisher whose queue size is 20, there should be no any blocking. So assert the elapsed time less than 10 ms.
	// You will also see 0~10 requests are dropped in Warning log (depends on the consuming speed vs ingestion speed)
	assert.True(t, time.Since(start) < 10*time.Millisecond)
	publisher.Close()
	// only 10 requests are published since the concurrency is 10
	assert.Equal(t, int32(10), c.counter)
}
