// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extract

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector/detectortest"
)

func TestPortExtractor(t *testing.T) {
	ctx := context.Background()
	extractor := NewPortExtractor()

	tests := map[string]struct {
		setup    func(*detectortest.MockProcess)
		wantPort int
	}{
		"Success/PortFromLongFlag": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--port", "3307"}, nil)
			},
			wantPort: 3307,
		},
		"Success/PortFromLongFlagEquals": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--port=3308"}, nil)
			},
			wantPort: 3308,
		},
		"Success/PortFromShortFlag": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "-P", "3309"}, nil)
			},
			wantPort: 3309,
		},
		"Success/PortFromShortFlagNoSpace": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "-P3310"}, nil)
			},
			wantPort: 3310,
		},
		"Success/PortFromEnv": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin", "MYSQL_TCP_PORT=3311"}, nil)
			},
			wantPort: 3311,
		},
		"Success/CmdlineTakesPrecedence": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--port", "3307"}, nil)
				mp.On("EnvironWithContext", ctx).Maybe().Return([]string{"PATH=/usr/bin", "MYSQL_TCP_PORT=3311"}, nil)
			},
			wantPort: 3307,
		},
		"Success/DefaultPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin"}, nil)
			},
			wantPort: 3306,
		},
		"Success/DefaultOnAllSourcesFail": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return(nil, assert.AnError)
				mp.On("EnvironWithContext", ctx).Return(nil, assert.AnError)
			},
			wantPort: 3306,
		},
		"Success/DefaultOnInvalidPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--port", "99999"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin"}, nil)
			},
			wantPort: 3306,
		},
		"Success/DefaultOnMalformedPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--port", "abc"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin"}, nil)
			},
			wantPort: 3306,
		},
		"Success/DefaultOnPort0": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--port", "0"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin"}, nil)
			},
			wantPort: 3306,
		},
		"Success/DefaultOnShortFlagInvalidPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "-P", "99999"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin"}, nil)
			},
			wantPort: 3306,
		},
		"Success/DefaultOnEqualsInvalidPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--port=99999"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin"}, nil)
			},
			wantPort: 3306,
		},
		"Success/DefaultOnEqualsMalformed": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--port=abc"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin"}, nil)
			},
			wantPort: 3306,
		},
		"Success/DefaultOnEqualsPort0": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--port=0"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin"}, nil)
			},
			wantPort: 3306,
		},
		"Success/DefaultOnShortFlagAttachedInvalid": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "-P99999"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin"}, nil)
			},
			wantPort: 3306,
		},
		"Success/DefaultOnShortFlagAttachedMalformed": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "-Pabc"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin"}, nil)
			},
			wantPort: 3306,
		},
		"Success/DefaultOnShortFlagAttachedPort0": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "-P0"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin"}, nil)
			},
			wantPort: 3306,
		},
		"Success/InvalidPortThenValidPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--port=abc", "--port=3307"}, nil)
			},
			wantPort: 3307,
		},
		"Success/Port0ThenValidPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--port", "0", "--port", "3307"}, nil)
			},
			wantPort: 3307,
		},
		"Success/ValidPortThenInvalid": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--port", "3307", "--port", "abc"}, nil)
			},
			wantPort: 3307,
		},
		"Success/InvalidEnvFallsBackToDefault": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"MYSQL_TCP_PORT=abc"}, nil)
			},
			wantPort: 3306,
		},
		"Success/Env0FallsBackToDefault": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"MYSQL_TCP_PORT=0"}, nil)
			},
			wantPort: 3306,
		},
		"Success/EnvInvalidPortFallsBackToDefault": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"MYSQL_TCP_PORT=99999"}, nil)
			},
			wantPort: 3306,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mp := new(detectortest.MockProcess)
			tt.setup(mp)

			port, err := extractor.Extract(ctx, mp)

			require.NoError(t, err)
			assert.Equal(t, tt.wantPort, port)
			mp.AssertExpectations(t)
		})
	}
}
