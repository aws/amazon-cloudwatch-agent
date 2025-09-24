// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package detectortest

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
)

type MockProcess struct {
	mock.Mock
}

var _ detector.Process = (*MockProcess)(nil)

func (m *MockProcess) PID() int32 {
	args := m.Called()
	return args.Get(0).(int32)
}

func (m *MockProcess) ExeWithContext(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockProcess) CwdWithContext(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockProcess) CmdlineSliceWithContext(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockProcess) EnvironWithContext(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

type MockProcessDetector struct {
	mock.Mock
}

var _ detector.ProcessDetector = (*MockProcessDetector)(nil)

func (m *MockProcessDetector) Detect(ctx context.Context, process detector.Process) (*detector.Metadata, error) {
	args := m.Called(ctx, process)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*detector.Metadata), args.Error(1)
}

type MockExtractor[T any] struct {
	mock.Mock
}

var _ detector.Extractor[any] = (*MockExtractor[any])(nil)

func (m *MockExtractor[T]) Extract(ctx context.Context, process detector.Process) (T, error) {
	args := m.Called(ctx, process)
	out, ok := args.Get(0).(T)
	if !ok {
		panic("Invalid return type for extractor")
	}
	return out, args.Error(1)
}
