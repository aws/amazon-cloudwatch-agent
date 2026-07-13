// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extract

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/detectortest"
)

func TestPortExtractor(t *testing.T) {
	tests := map[string]struct {
		setup   func(*detectortest.MockProcess)
		want    int
		wantErr error
	}{
		"WithPort/Cmdline": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", context.Background()).
					Return([]string{"java", "-Dcom.sun.management.jmxremote.port=1234"}, nil)
			},
			want: 1234,
		},
		"WithPort/Env": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", context.Background()).
					Return([]string{"java", "-Dcom.sun.management.jmxremote.port=invalid_port"}, nil)
				mp.On("EnvironWithContext", context.Background()).
					Return([]string{"JMX_PORT=2345"}, nil)
			},
			want: 2345,
		},
		"WithNoPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", context.Background()).
					Return([]string{"java"}, nil)
				mp.On("EnvironWithContext", context.Background()).
					Return([]string{}, nil)
			},
			want:    -1,
			wantErr: detector.ErrExtractPort,
		},
		"WithInvalidPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", context.Background()).
					Return([]string{"java"}, nil)
				mp.On("EnvironWithContext", context.Background()).
					Return([]string{"JMX_PORT=1111111"}, nil)
			},
			want:    -1,
			wantErr: detector.ErrInvalidPort,
		},
		"WithProcessError": {
			setup: func(m *detectortest.MockProcess) {
				m.On("CmdlineSliceWithContext", context.Background()).
					Return(nil, assert.AnError)
				m.On("EnvironWithContext", context.Background()).
					Return(nil, assert.AnError)
			},
			want:    -1,
			wantErr: assert.AnError,
		},
	}
	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			mp := new(detectortest.MockProcess)
			testCase.setup(mp)

			extractor := NewPortExtractor()
			got, err := extractor.Extract(context.Background(), mp)
			if testCase.wantErr != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, testCase.want, got)
			mp.AssertExpectations(t)
		})
	}
}
