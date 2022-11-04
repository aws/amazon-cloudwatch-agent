package atomiccounter

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAtomicCounter(t *testing.T) {
	x := NewAtomicCounter()
	assert.Equal(t, int64(0), x.Get())

	for i := int64(0); i < 1000; i++ {
		assert.Equal(t, i, x.Get())
		x.Increment()
	}

	for i := int64(1000 - 1); i >= 1000; i-- {
		assert.Equal(t, i, x.Get())
		x.Decrement()
	}
}

// TestAtomicCounterParallel runs many goroutines to inc and dec the same
// amount of times so that the expected end result is counter == 0.
func TestAtomicCounterParallel(t *testing.T) {
	x := NewAtomicCounter()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			for j := 0; j < 100; j++ {
				x.Increment()
			}
			wg.Done()
		}()
		go func () {
			for k := 0; k < 100; k++ {
				x.Decrement()
			}
			wg.Done()
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(0), x.Get())
}