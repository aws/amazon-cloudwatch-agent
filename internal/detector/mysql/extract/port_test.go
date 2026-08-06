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
