// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package postgresql

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/detectortest"
)

func TestDetect(t *testing.T) {
	ctx := context.Background()
	testCases := map[string]struct {
		setup   func(*detectortest.MockProcess)
		want    *detector.Metadata
		wantErr error
	}{
		"Success/PostgreSQL_DefaultPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/bin/postgres", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"postgres"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{}, nil)
			},
			want: &detector.Metadata{
				Name:          "postgresql",
				Categories:    []detector.Category{detector.CategoryPostgreSQL},
				Status:        detector.StatusReady,
				TelemetryPort: 5432,
			},
		},
		"Success/PostgreSQL_CustomPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/bin/postgres", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"postgres", "-p", "5433"}, nil)
			},
			want: &detector.Metadata{
				Name:          "postgresql",
				Categories:    []detector.Category{detector.CategoryPostgreSQL},
				Status:        detector.StatusReady,
				TelemetryPort: 5433,
			},
		},
		"Success/PostgreSQL_PortFromEnv": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/bin/postgres", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"postgres", "-D", "/data"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"PGPORT=5434"}, nil)
			},
			want: &detector.Metadata{
				Name:          "postgresql",
				Categories:    []detector.Category{detector.CategoryPostgreSQL},
				Status:        detector.StatusReady,
				TelemetryPort: 5434,
			},
		},
		"Success/PostgreSQL_UsrLocal": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/local/pgsql/bin/postgres", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"postgres", "-D", "/data"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{}, nil)
			},
			want: &detector.Metadata{
				Name:          "postgresql",
				Categories:    []detector.Category{detector.CategoryPostgreSQL},
				Status:        detector.StatusReady,
				TelemetryPort: 5432,
			},
		},
		"Incompatible/WorkerProcess_IOWorker": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/bin/postgres", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"postgres: io worker 0"}, nil)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
		"Incompatible/WorkerProcess_Checkpointer": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/bin/postgres", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"postgres: checkpointer"}, nil)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
		"Incompatible/MySQL": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/bin/mysqld", nil)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
		"Error/ExeWithContext": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("", assert.AnError)
			},
			wantErr: assert.AnError,
		},
		"Error/CmdlineSliceWithContext": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/bin/postgres", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return(nil, assert.AnError)
			},
			wantErr: assert.AnError,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mp := new(detectortest.MockProcess)
			testCase.setup(mp)

			d := NewDetector(slog.Default())
			got, err := d.Detect(ctx, mp)

			if testCase.wantErr != nil {
				require.ErrorIs(t, err, testCase.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCase.want, got)
			}
			mp.AssertExpectations(t)
		})
	}
}
