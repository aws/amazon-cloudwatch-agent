package publisher

// It is a FIFO queue with the functionality that block the caller if the queue size reaches to the maxSize
type BlockingFifoQueue struct {
	queue chan interface{}
}

func NewBlockingFifoQueue(size int) *BlockingFifoQueue {
	if size <= 0 {
		panic("Queue Size should be larger than 0!")
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
