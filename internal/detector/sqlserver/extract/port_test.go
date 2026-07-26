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
		"Success/PortFromShortFlag": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"sqlservr", "-p", "1434"}, nil)
			},
			wantPort: 1434,
		},
		"Success/PortFromShortFlagNoSpace": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"sqlservr", "-p1435"}, nil)
			},
			wantPort: 1435,
		},
		"Success/PortFromEnv": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"sqlservr"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin", "MSSQL_TCP_PORT=1436"}, nil)
			},
			wantPort: 1436,
		},
		"Success/CmdlineTakesPrecedence": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"sqlservr", "-p", "1437"}, nil)
			},
			wantPort: 1437,
		},
		"Success/DefaultPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"sqlservr"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin"}, nil)
			},
			wantPort: 1433,
		},
		"Success/DefaultOnAllSourcesFail": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return(nil, assert.AnError)
				mp.On("EnvironWithContext", ctx).Return(nil, assert.AnError)
			},
			wantPort: 1433,
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
