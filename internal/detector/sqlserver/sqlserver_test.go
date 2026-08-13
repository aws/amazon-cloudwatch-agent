// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package sqlserver

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
	logger := slog.Default()
	d := NewDetector(logger)

	tests := map[string]struct {
		setup   func(*detectortest.MockProcess)
		want    *detector.Metadata
		wantErr error
	}{
		"Success/DefaultPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/opt/mssql/bin/sqlservr", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"sqlservr"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{}, nil)
			},
			want: &detector.Metadata{
				Name:          "sqlserver",
				Categories:    []detector.Category{detector.CategorySQLServer},
				Status:        detector.StatusReady,
				TelemetryPort: 1433,
			},
		},
		"Success/CustomPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/opt/mssql/bin/sqlservr", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"sqlservr", "-p", "1434"}, nil)
			},
			want: &detector.Metadata{
				Name:          "sqlserver",
				Categories:    []detector.Category{detector.CategorySQLServer},
				Status:        detector.StatusReady,
				TelemetryPort: 1434,
			},
		},
		"Success/PortFromEnv": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/opt/mssql/bin/sqlservr", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"sqlservr"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"MSSQL_TCP_PORT=1435"}, nil)
			},
			want: &detector.Metadata{
				Name:          "sqlserver",
				Categories:    []detector.Category{detector.CategorySQLServer},
				Status:        detector.StatusReady,
				TelemetryPort: 1435,
			},
		},
		"Success/AlternateInstallPath": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/local/mssql/bin/sqlservr", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"sqlservr"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{}, nil)
			},
			want: &detector.Metadata{
				Name:          "sqlserver",
				Categories:    []detector.Category{detector.CategorySQLServer},
				Status:        detector.StatusReady,
				TelemetryPort: 1433,
			},
		},
		"Success/DefaultPortWithOtherFlags": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/opt/mssql/bin/sqlservr", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"sqlservr", "-d", "/var/opt/mssql/data"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{}, nil)
			},
			want: &detector.Metadata{
				Name:          "sqlserver",
				Categories:    []detector.Category{detector.CategorySQLServer},
				Status:        detector.StatusReady,
				TelemetryPort: 1433,
			},
		},
		"Incompatible/NotSQLServer": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/bin/postgres", nil)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
		"Incompatible/SQLCmd": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/bin/sqlcmd", nil)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
		"Error/ExeWithContext": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("", assert.AnError)
			},
			wantErr: assert.AnError,
		},
		"Success/DefaultPortOnCmdlineError": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/opt/mssql/bin/sqlservr", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return(nil, assert.AnError)
				mp.On("EnvironWithContext", ctx).Return(nil, assert.AnError)
			},
			want: &detector.Metadata{
				Name:          "sqlserver",
				Categories:    []detector.Category{detector.CategorySQLServer},
				Status:        detector.StatusReady,
				TelemetryPort: 1433,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mp := new(detectortest.MockProcess)
			tt.setup(mp)

			got, err := d.Detect(ctx, mp)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
			mp.AssertExpectations(t)
		})
	}
}
