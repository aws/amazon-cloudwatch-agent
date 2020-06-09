package publisher

import (
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
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

func TestPublisher_PublishWithNonBlockingLifoQueue(t *testing.T) {
	c := &testClient{}
	publisher, _ := NewPublisher(NewNonBlockingLifoQueue(2), 1, 2*time.Second, c.publish)
	publisher.Publish("req1")
	publisher.Publish("req2")
	publisher.Close()
	assert.Equal(t, []string{"req2", "req1"}, c.getResult())
}

func TestPublisher_PublishWithNonBlockingLifoQueueSleep(t *testing.T) {
	c := &testClient{}
	publisher, _ := NewPublisher(NewNonBlockingLifoQueue(2), 1, 2*time.Second, c.publish)
	publisher.Publish("req1")
	time.Sleep(100 * time.Millisecond)
	publisher.Publish("req2")
	publisher.Close()
	assert.Equal(t, []string{"req1", "req2"}, c.getResult())
}

func TestPublisher_PublishWithBlockingFifoQueue(t *testing.T) {
	c := &testClient{}
	publisher, _ := NewPublisher(NewBlockingFifoQueue(1), 1, 2*time.Second, c.publishWith1sLatency)
	start := time.Now()
	publisher.Publish("req1")
	publisher.Publish("req2")
	publisher.Publish("req3")
	publisher.Publish("req4")
	// queue size is 1, so the forth publish should block util the first req is published( because then second req get semphore, and third req get dequeued)
	assert.True(t, time.Now().Sub(start) > 1*time.Second)
	time.Sleep(3 * time.Second)
	publisher.Close()
	assert.Equal(t, []string{"req1", "req2", "req3", "req4"}, c.getResult())
}

func TestPublisher_DrainTimeout(t *testing.T) {
	start := time.Now()
	c := &testClient{}
	publisher, _ := NewPublisher(NewBlockingFifoQueue(2), 1, 2*time.Second, c.publishWith5sLatency)
	publisher.Publish("req1")
	publisher.Publish("req2")
	publisher.Close()
	// drain queue timeout 2s + on fly request timeout 1s = 3s (expected)
	assert.True(t, time.Now().Sub(start) < 4*time.Second)
}

// testClientNoMutex is to test whether need memory barrier when concurrency of publisher is 1
type testClientNoMutex struct {
	counter int
}

func (c *testClientNoMutex) publish(req interface{}) {
	r := req.(int)
	c.counter += r
}

func TestPublisher_ClientNoMutex(t *testing.T) {
	c := &testClientNoMutex{}
	publisher, _ := NewPublisher(NewBlockingFifoQueue(10), 1, 2*time.Second, c.publish)
	for i := 0; i < 100000; i++ {
		publisher.Publish(1)
	}
	publisher.Close()
	assert.Equal(t, 100000, c.counter)
}

// testClientLongDelay is to test publisher latency with nonBlockingQueue
type testClientLongDelay struct {
	counter int
}

func (c *testClientLongDelay) publish(req interface{}) {
	r := req.(int)
	c.counter += r
	time.Sleep(time.Minute)
}
func TestPublisher_ClientLongDelay(t *testing.T) {
	c := &testClientLongDelay{}
	publisher, _ := NewPublisher(NewNonBlockingFifoQueue(20), 10, 5*time.Second, c.publish)
	start := time.Now()
	for i := 0; i < 30; i++ {
		publisher.Publish(1)
	}
	// Send 30 requests to the publisher whose queue size is 20, there should be no any blocking. So assert the elapsed time less than 10 ms.
	// You will also see 0~10 requests are dropped in Warning log (depends on the consuming speed vs ingestion speed)
	assert.True(t, time.Now().Sub(start) < 10*time.Millisecond)
	publisher.Close()
	// only 10 requests are published since the concurrency is 10
	assert.Equal(t, 10, c.counter)
}
