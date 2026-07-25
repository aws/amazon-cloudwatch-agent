// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statetest

import (
	"runtime"
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
	time.Sleep(time.Millisecond)
	close(done)
	wg.Wait()

	got, err := sink.Restore()
	// baseline-instrument: capture the sync state at the boundary. GetSink is
	// updated synchronously in Enqueue (always 2), so if got has < 2 ranges
	// the failure is in rangeManager.Run's shutdown race (Go's select picked
	// <-Done while items were still pending in the internal queue). If
	// GetSink itself is < 2 something's wrong at the sink layer. Also log
	// goroutines/GOMAXPROCS for load context.
	t.Logf("baseline-instrument: TestNewFileManagerSink getSink_len=%d restored_len=%d "+
		"err=%v goroutines=%d GOMAXPROCS=%d",
		len(sink.GetSink()), len(got), err, runtime.NumGoroutine(), runtime.GOMAXPROCS(0))
	assert.NoError(t, err)
	assert.Equal(t, state.RangeList{
		state.NewRange(0, 10),
	}, got)

	assert.Equal(t, state.RangeList{
		state.NewRange(0, 5),
		state.NewRange(5, 10),
	}, sink.GetSink())
}
