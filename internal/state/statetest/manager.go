// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statetest

import "github.com/aws/amazon-cloudwatch-agent/internal/state"

type FileRangeManagerSink struct {
	base state.FileRangeManager
	sink state.RangeList
}

var _ state.FileRangeManager = (*FileRangeManagerSink)(nil)

func NewFileManagerSink(base state.FileRangeManager) *FileRangeManagerSink {
	return &FileRangeManagerSink{
		base: base,
		sink: make(state.RangeList, 0),
	}
}

func (f *FileRangeManagerSink) ID() string {
	return f.base.ID()
}

func (f *FileRangeManagerSink) Enqueue(r state.Range) {
	f.sink = append(f.sink, r)
	f.base.Enqueue(r)
}

func (f *FileRangeManagerSink) Restore() (state.RangeList, error) {
	return f.base.Restore()
}

func (f *FileRangeManagerSink) Run(ch state.Notification) {
	f.base.Run(ch)
}

func (f *FileRangeManagerSink) GetSink() state.RangeList {
	return f.sink
}
