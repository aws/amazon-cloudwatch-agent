// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"context"
	"testing"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector/detectortest"
)

func TestNewCachedProcess_PID(t *testing.T) {
	tp := &process.Process{Pid: int32(1234)}
	cached := NewCachedProcess(NewProcessWithPID(tp))
	assert.Equal(t, int32(1234), cached.PID())
}

func TestCachedProcess_ExeWithContext(t *testing.T) {
	ctx := context.Background()

	testCases := map[string]struct {
		setupMock func(*detectortest.MockProcess)
		wantExe   string
		wantErr   error
	}{
		"WithSuccess": {
			setupMock: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/path/to/exe", nil).Once()
			},
			wantExe: "/path/to/exe",
		},
		"WithError": {
			setupMock: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("", assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mp := new(detectortest.MockProcess)
			testCase.setupMock(mp)

			cached := NewCachedProcess(mp)

			got, err := cached.ExeWithContext(ctx)
			assert.Equal(t, testCase.wantExe, got)
			assert.Equal(t, testCase.wantErr, err)

			got, err = cached.ExeWithContext(ctx)
			assert.Equal(t, testCase.wantExe, got)
			assert.Equal(t, testCase.wantErr, err)

			mp.AssertExpectations(t)
			mp.AssertNumberOfCalls(t, "ExeWithContext", 1)
		})
	}
}

func TestCachedProcess_CwdWithContext(t *testing.T) {
	ctx := context.Background()

	testCases := map[string]struct {
		name      string
		setupMock func(*detectortest.MockProcess)
		wantCwd   string
		wantErr   error
	}{
		"WithSuccess": {
			setupMock: func(mp *detectortest.MockProcess) {
				mp.On("CwdWithContext", ctx).Return("/working/dir", nil).Once()
			},
			wantCwd: "/working/dir",
		},
		"WithError": {
			setupMock: func(mp *detectortest.MockProcess) {
				mp.On("CwdWithContext", ctx).Return("", assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mp := new(detectortest.MockProcess)
			testCase.setupMock(mp)

			cached := NewCachedProcess(mp)

			got, err := cached.CwdWithContext(ctx)
			assert.Equal(t, testCase.wantCwd, got)
			assert.Equal(t, testCase.wantErr, err)

			got, err = cached.CwdWithContext(ctx)
			assert.Equal(t, testCase.wantCwd, got)
			assert.Equal(t, testCase.wantErr, err)

			mp.AssertExpectations(t)
			mp.AssertNumberOfCalls(t, "CwdWithContext", 1)
		})
	}
}

func TestCachedProcess_CmdlineSliceWithContext(t *testing.T) {
	ctx := context.Background()

	testCases := map[string]struct {
		name             string
		setupMock        func(*detectortest.MockProcess)
		wantCmdlineSlice []string
		wantErr          error
	}{
		"WithSuccess": {
			setupMock: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"cmd", "-arg1", "-arg2"}, nil).Once()
			},
			wantCmdlineSlice: []string{"cmd", "-arg1", "-arg2"},
		},
		"WithError": {
			setupMock: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return(nil, assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mp := new(detectortest.MockProcess)
			testCase.setupMock(mp)

			cached := NewCachedProcess(mp)

			got, err := cached.CmdlineSliceWithContext(ctx)
			assert.Equal(t, testCase.wantCmdlineSlice, got)
			assert.Equal(t, testCase.wantErr, err)

			got, err = cached.CmdlineSliceWithContext(ctx)
			assert.Equal(t, testCase.wantCmdlineSlice, got)
			assert.Equal(t, testCase.wantErr, err)

			mp.AssertExpectations(t)
			mp.AssertNumberOfCalls(t, "CmdlineSliceWithContext", 1)
		})
	}
}

func TestCachedProcess_EnvironWithContext(t *testing.T) {
	ctx := context.Background()

	testCases := map[string]struct {
		name             string
		setupMock        func(*detectortest.MockProcess)
		wantCmdlineSlice []string
		wantErr          error
	}{
		"WithSuccess": {
			setupMock: func(mp *detectortest.MockProcess) {
				mp.On("EnvironWithContext", ctx).Return([]string{"cmd", "-arg1", "-arg2"}, nil).Once()
			},
			wantCmdlineSlice: []string{"cmd", "-arg1", "-arg2"},
		},
		"WithError": {
			setupMock: func(mp *detectortest.MockProcess) {
				mp.On("EnvironWithContext", ctx).Return(nil, assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mp := new(detectortest.MockProcess)
			testCase.setupMock(mp)

			cached := NewCachedProcess(mp)

			got, err := cached.EnvironWithContext(ctx)
			assert.Equal(t, testCase.wantCmdlineSlice, got)
			assert.Equal(t, testCase.wantErr, err)

			got, err = cached.EnvironWithContext(ctx)
			assert.Equal(t, testCase.wantCmdlineSlice, got)
			assert.Equal(t, testCase.wantErr, err)

			mp.AssertExpectations(t)
			mp.AssertNumberOfCalls(t, "EnvironWithContext", 1)
		})
	}
}

func TestCachedProcess_WithContextError(t *testing.T) {
	mp := new(detectortest.MockProcess)
	cached := NewCachedProcess(mp)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := cached.ExeWithContext(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)

	_, err = cached.CwdWithContext(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)

	_, err = cached.CmdlineSliceWithContext(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)

	_, err = cached.EnvironWithContext(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)

	mp.AssertNotCalled(t, "ExeWithContext")
	mp.AssertNotCalled(t, "CwdWithContext")
	mp.AssertNotCalled(t, "CmdlineSliceWithContext")
	mp.AssertNotCalled(t, "EnvironWithContext")
}
