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
		"Success/PortFromFlag": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"postgres", "-p", "5433"}, nil)
			},
			wantPort: 5433,
		},
		"Success/PortFromFlagNoSpace": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"postgres", "-p5434"}, nil)
			},
			wantPort: 5434,
		},
		"Success/PortFromEnv": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"postgres"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin", "PGPORT=5435"}, nil)
			},
			wantPort: 5435,
		},
		// cmdline is tried first; when it finds a port, env is never called
		"Success/CmdlineTakesPrecedence": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"postgres", "-p", "5433"}, nil)
			},
			wantPort: 5433,
		},
		"Success/DefaultPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"postgres"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PATH=/usr/bin"}, nil)
			},
			wantPort: 5432,
		},
		"Success/DefaultPortWithOtherFlags": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"postgres", "-D", "/var/lib/postgresql/data"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{}, nil)
			},
			wantPort: 5432,
		},
		"Success/DefaultOnAllSourcesFail": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return(nil, assert.AnError)
				mp.On("EnvironWithContext", ctx).Return(nil, assert.AnError)
			},
			wantPort: 5432,
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
