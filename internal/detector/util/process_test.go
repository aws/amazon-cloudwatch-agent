// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util //nolint:revive // existing package name

import (
	"context"
	"testing"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
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
		name        string
		setupMock   func(*detectortest.MockProcess)
		wantEnviron []string
		wantErr     error
	}{
		"WithSuccess": {
			setupMock: func(mp *detectortest.MockProcess) {
				mp.On("EnvironWithContext", ctx).Return([]string{"KEY=VALUE", "K1=V1"}, nil).Once()
			},
			wantEnviron: []string{"KEY=VALUE", "K1=V1"},
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
			assert.Equal(t, testCase.wantEnviron, got)
			assert.Equal(t, testCase.wantErr, err)

			got, err = cached.EnvironWithContext(ctx)
			assert.Equal(t, testCase.wantEnviron, got)
			assert.Equal(t, testCase.wantErr, err)

			mp.AssertExpectations(t)
			mp.AssertNumberOfCalls(t, "EnvironWithContext", 1)
		})
	}
}

func TestCachedProcess_CreateTimeWithContext(t *testing.T) {
	ctx := context.Background()

	testCases := map[string]struct {
		name           string
		setupMock      func(*detectortest.MockProcess)
		wantCreateTime int64
		wantErr        error
	}{
		"WithSuccess": {
			setupMock: func(mp *detectortest.MockProcess) {
				mp.On("CreateTimeWithContext", ctx).Return(1000, nil).Once()
			},
			wantCreateTime: int64(1000),
		},
		"WithError": {
			setupMock: func(mp *detectortest.MockProcess) {
				mp.On("CreateTimeWithContext", ctx).Return(0, assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mp := new(detectortest.MockProcess)
			testCase.setupMock(mp)

			cached := NewCachedProcess(mp)

			got, err := cached.CreateTimeWithContext(ctx)
			assert.Equal(t, testCase.wantCreateTime, got)
			assert.Equal(t, testCase.wantErr, err)

			got, err = cached.CreateTimeWithContext(ctx)
			assert.Equal(t, testCase.wantCreateTime, got)
			assert.Equal(t, testCase.wantErr, err)

			mp.AssertExpectations(t)
			mp.AssertNumberOfCalls(t, "CreateTimeWithContext", 1)
		})
	}
}

func TestCachedProcess_OpenFilesWithContext(t *testing.T) {
	ctx := context.Background()

	testCases := map[string]struct {
		name          string
		setupMock     func(*detectortest.MockProcess)
		wantOpenFiles []detector.OpenFilesStat
		wantErr       error
	}{
		"WithSuccess": {
			setupMock: func(mp *detectortest.MockProcess) {
				mp.On("OpenFilesWithContext", ctx).Return([]detector.OpenFilesStat{{
					Path: "/some/file/path",
					Fd:   10,
				}}, nil).Once()
			},
			wantOpenFiles: []detector.OpenFilesStat{{
				Path: "/some/file/path",
				Fd:   10,
			}},
		},
		"WithError": {
			setupMock: func(mp *detectortest.MockProcess) {
				mp.On("OpenFilesWithContext", ctx).Return(nil, assert.AnError).Once()
			},
			wantErr: assert.AnError,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mp := new(detectortest.MockProcess)
			testCase.setupMock(mp)

			cached := NewCachedProcess(mp)

			got, err := cached.OpenFilesWithContext(ctx)
			assert.Equal(t, testCase.wantOpenFiles, got)
			assert.Equal(t, testCase.wantErr, err)

			got, err = cached.OpenFilesWithContext(ctx)
			assert.Equal(t, testCase.wantOpenFiles, got)
			assert.Equal(t, testCase.wantErr, err)

			mp.AssertExpectations(t)
			mp.AssertNumberOfCalls(t, "OpenFilesWithContext", 1)
		})
	}
}
