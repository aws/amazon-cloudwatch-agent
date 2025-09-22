// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/detectortest"
)

func TestDiscoverer_detectMetadataFromProcesses(t *testing.T) {
	testCases := map[string]struct {
		setupMock func(*detectortest.MockProcessDetector)
		want      []*detector.Metadata
		wantErr   error
	}{
		"SingleProcess": {
			setupMock: func(md *detectortest.MockProcessDetector) {
				md.On("Detect", mock.Anything, mock.Anything).Return(&detector.Metadata{
					Categories: []detector.Category{detector.CategoryJVM},
					Name:       "test-process",
				}, nil).Once()
			},
			want: []*detector.Metadata{
				{
					Categories: []detector.Category{detector.CategoryJVM},
					Name:       "test-process",
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			md := new(detectortest.MockProcessDetector)
			testCase.setupMock(md)

			cfg := Config{
				LogLevel:    slog.LevelDebug,
				Concurrency: 1,
				Timeout:     time.Second,
			}
			d := NewDiscoverer(cfg, slog.Default())
			d.processDetectors = []detector.ProcessDetector{md}

			ctx := context.Background()
			got, err := d.detectMetadataFromProcesses(ctx, []*process.Process{
				{Pid: int32(1234)},
				{Pid: int32(os.Getpid())}, // nolint:gosec
			})
			if testCase.wantErr != nil {
				assert.ErrorIs(t, err, testCase.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, detector.MetadataSlice(testCase.want), got)
			}
			md.AssertExpectations(t)
		})
	}
}

func TestDiscoverer_detectMetadataFromProcess(t *testing.T) {
	testCases := map[string]struct {
		setupMock func(*detectortest.MockProcessDetector)
		want      []*detector.Metadata
		wantErr   error
	}{
		"SkipProcess": {
			setupMock: func(md *detectortest.MockProcessDetector) {
				md.On("Detect", mock.Anything, mock.Anything).
					Return(nil, detector.ErrSkipProcess).Once()
			},
			wantErr: detector.ErrSkipProcess,
		},
		"NoCompatibleDetectors": {
			setupMock: func(m *detectortest.MockProcessDetector) {
				m.On("Detect", mock.Anything, mock.Anything).
					Return(nil, detector.ErrIncompatibleDetector).Once()
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			md := new(detectortest.MockProcessDetector)
			testCase.setupMock(md)

			cfg := Config{
				LogLevel:    slog.LevelDebug,
				Concurrency: 1,
				Timeout:     time.Second,
			}
			d := NewDiscoverer(cfg, slog.Default())
			d.processDetectors = []detector.ProcessDetector{md}

			mp := new(detectortest.MockProcess)

			ctx := context.Background()
			got, err := d.detectMetadataFromProcess(ctx, mp)
			if testCase.wantErr != nil {
				assert.ErrorIs(t, err, testCase.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, detector.MetadataSlice(testCase.want), got)
			}
			md.AssertExpectations(t)
		})
	}
}
