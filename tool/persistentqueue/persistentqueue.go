package persistentqueue

type PersistentQueue interface {
	Enqueue(interface{}) error
	Dequeue() (interface{}, error)
	Depth() int64
	Close() error
}

type Marshaler func(interface{}) ([]byte, error)
type Unmarshaler func([]byte) (interface{}, error)
