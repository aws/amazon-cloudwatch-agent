// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mysql

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
				mp.On("ExeWithContext", ctx).Return("/usr/sbin/mysqld", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{}, nil)
			},
			want: &detector.Metadata{
				Name:          "mysql",
				Categories:    []detector.Category{detector.CategoryMySQL},
				Status:        detector.StatusReady,
				TelemetryPort: 3306,
			},
		},
		"Success/CustomPort": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/sbin/mysqld", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--port", "3307"}, nil)
			},
			want: &detector.Metadata{
				Name:          "mysql",
				Categories:    []detector.Category{detector.CategoryMySQL},
				Status:        detector.StatusReady,
				TelemetryPort: 3307,
			},
		},
		"Success/PortFromEnv": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/sbin/mysqld", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--datadir=/var/lib/mysql"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"MYSQL_TCP_PORT=3308"}, nil)
			},
			want: &detector.Metadata{
				Name:          "mysql",
				Categories:    []detector.Category{detector.CategoryMySQL},
				Status:        detector.StatusReady,
				TelemetryPort: 3308,
			},
		},
		"Success/AlternateInstallPath": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/local/mysql/bin/mysqld", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--datadir=/var/lib/mysql"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{}, nil)
			},
			want: &detector.Metadata{
				Name:          "mysql",
				Categories:    []detector.Category{detector.CategoryMySQL},
				Status:        detector.StatusReady,
				TelemetryPort: 3306,
			},
		},
		"Success/DefaultPortWithOtherFlags": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/sbin/mysqld", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"mysqld", "--datadir=/var/lib/mysql", "--user=mysql", "--socket=/var/run/mysqld/mysqld.sock"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{}, nil)
			},
			want: &detector.Metadata{
				Name:          "mysql",
				Categories:    []detector.Category{detector.CategoryMySQL},
				Status:        detector.StatusReady,
				TelemetryPort: 3306,
			},
		},
		"Incompatible/NotMySQL": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/bin/postgres", nil)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
		"Incompatible/MySQLClient": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("ExeWithContext", ctx).Return("/usr/bin/mysql", nil)
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
				mp.On("ExeWithContext", ctx).Return("/usr/sbin/mysqld", nil)
				mp.On("CmdlineSliceWithContext", ctx).Return(nil, assert.AnError)
				mp.On("EnvironWithContext", ctx).Return(nil, assert.AnError)
			},
			want: &detector.Metadata{
				Name:          "mysql",
				Categories:    []detector.Category{detector.CategoryMySQL},
				Status:        detector.StatusReady,
				TelemetryPort: 3306,
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
