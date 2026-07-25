// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statetest

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/state"
)

func TestNewFileManagerSink(t *testing.T) {
	tmpDir := t.TempDir()
	sink := NewFileManagerSink(state.NewFileRangeManager(state.ManagerConfig{
		StateFileDir:      tmpDir,
		Name:              "sink",
		MaxPersistedItems: 1,
	}))
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sink.Run(state.Notification{Done: done})
	}()
	assert.Equal(t, "sink", sink.ID())
	sink.Enqueue(state.NewRange(0, 5))
	sink.Enqueue(state.NewRange(5, 10))
	// Give the Run goroutine time to drain both queued ranges before we
	// signal shutdown. The original code slept exactly time.Millisecond
	// (1 ms) -- smaller than Windows' ~15.6 ms timer tick, so under
	// `make test` CPU contention the goroutine sometimes hadn't been
	// scheduled at all when close(done) fired, and the underlying
	// rangeManager.Run's select non-deterministically picked <-Done
	// while queue items were still pending, saving an empty state to
	// disk. Observed run 30124545948 baseline iter 25 as
	//   expected: state.RangeList{{start:0, end:10, seq:0}}
	//   actual:   state.RangeList{{start:0, end:0,  seq:0}}
	// 200 ms is ~13 Windows ticks, plenty for two channel-send items
	// to be consumed. The happy-path runtime overhead is negligible.
	time.Sleep(200 * time.Millisecond)
	close(done)
	wg.Wait()

	got, err := sink.Restore()
	assert.NoError(t, err)
	assert.Equal(t, state.RangeList{
		state.NewRange(0, 10),
	}, got)

	assert.Equal(t, state.RangeList{
		state.NewRange(0, 5),
		state.NewRange(5, 10),
	}, sink.GetSink())
}
