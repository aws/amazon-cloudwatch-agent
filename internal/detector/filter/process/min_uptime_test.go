// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package process

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector/detectortest"
)

type mockTimeSince struct {
	mock.Mock
}

func (m *mockTimeSince) Since(t time.Time) time.Duration {
	args := m.Called(t)
	return args.Get(0).(time.Duration)
}

func TestMinUptimeFilter(t *testing.T) {
	type mocks struct {
		process   *detectortest.MockProcess
		timeSince *mockTimeSince
	}

	ctx := context.Background()
	testCases := map[string]struct {
		setup     func(m *mocks)
		minUptime time.Duration
		want      bool
	}{
		"Process/Error": {
			setup: func(m *mocks) {
				m.process.On("CreateTimeWithContext", ctx).Return(0, assert.AnError)
			},
			minUptime: time.Duration(0),
			want:      true,
		},
		"IncludeUptime": {
			setup: func(m *mocks) {
				m.process.On("CreateTimeWithContext", ctx).Return(10, nil)
				m.timeSince.On("Since", mock.Anything).Return(time.Minute)
			},
			minUptime: time.Minute,
			want:      true,
		},
		"ExcludeUptime": {
			setup: func(m *mocks) {
				m.process.On("CreateTimeWithContext", ctx).Return(10, nil)
				m.timeSince.On("Since", mock.Anything).Return(time.Second)
			},
			minUptime: time.Minute,
			want:      false,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			m := &mocks{
				process:   new(detectortest.MockProcess),
				timeSince: new(mockTimeSince),
			}
			testCase.setup(m)

			f := NewMinUptimeFilter(slog.Default(), testCase.minUptime)
			muf, ok := f.(*minUptimeFilter)
			require.True(t, ok)
			muf.timeSince = m.timeSince.Since
			assert.Equal(t, testCase.want, f.ShouldInclude(ctx, m.process))
		})
	}
}
