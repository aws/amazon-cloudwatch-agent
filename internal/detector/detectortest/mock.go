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
	return 1234
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

func (m *MockProcess) CreateTimeWithContext(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return int64(args.Int(0)), args.Error(1)
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

type MockDeviceDetector struct {
	mock.Mock
}

var _ detector.DeviceDetector = (*MockDeviceDetector)(nil)

func (m *MockDeviceDetector) Detect() (*detector.Metadata, error) {
	args := m.Called()
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

type MockProcessFilter struct {
	mock.Mock
}

var _ detector.ProcessFilter = (*MockProcessFilter)(nil)

func (m *MockProcessFilter) ShouldInclude(ctx context.Context, process detector.Process) bool {
	args := m.Called(ctx, process)
	return args.Bool(0)
}

type MockNameFilter struct {
	mock.Mock
}

var _ detector.NameFilter = (*MockNameFilter)(nil)

func (m *MockNameFilter) ShouldInclude(name string) bool {
	args := m.Called(name)
	return args.Bool(0)
}
