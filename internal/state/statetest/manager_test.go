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
	time.Sleep(time.Millisecond)
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
