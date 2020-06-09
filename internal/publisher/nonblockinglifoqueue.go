package publisher

import (
	"log"
	"sync"
)

// It is a LIFO queue with the functionality that dropping the tail if the queue size reaches to the maxSize
type NonBlockingLifoQueue struct {
	head    *node
	tail    *node
	length  int
	maxSize int
	sync.Mutex
}

type node struct {
	value interface{}
	prev  *node
	next  *node
}

func NewNonBlockingLifoQueue(size int) *NonBlockingLifoQueue {
	if size <= 0 {
		panic("Queue Size should be larger than 0!")
	}
	return &NonBlockingLifoQueue{maxSize: size}
}

func (u *NonBlockingLifoQueue) Dequeue() (interface{}, bool) {
	u.Lock()
	defer u.Unlock()
	if u.length == 0 {
		return nil, false
	}

	n := u.head
	if n.prev != nil {
		n.prev.next = nil
		u.head = n.prev
	} else {
		// last element is removed
		u.head = nil
		u.tail = nil
	}
	u.length--
	return n.value, true
}

// enqueue to the head of the queue, delete the tail if the queue has already reached to maxSize
func (u *NonBlockingLifoQueue) Enqueue(value interface{}) {
	u.Lock()
	defer u.Unlock()

	n := &node{value, u.head, nil}
	if u.head != nil {
		u.head.next = n
	}
	u.head = n

	if u.length == 0 {
		u.tail = n
	}

	if u.length == u.maxSize {
		log.Printf("W! message is dropped due to nonblocking lifo queue is full")
		// u.tail.next should not be nil
		u.tail.next.prev = nil
		u.tail = u.tail.next
	} else {
		u.length++
	}
}
