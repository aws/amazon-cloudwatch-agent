// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package kafkaclient

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/detectortest"
)

func TestKafkaClientDetector(t *testing.T) {
	ctx := context.Background()
	testCases := map[string]struct {
		setup   func(*detectortest.MockProcess)
		want    *detector.Metadata
		wantErr error
	}{
		"Process/Error": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return(nil, assert.AnError)
				mp.On("OpenFilesWithContext", ctx).Return(nil, assert.AnError)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
		"Process/NoKafkaClients": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"java"}, nil)
				mp.On("OpenFilesWithContext", ctx).Return([]detector.OpenFilesStat{}, nil)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
		"Process/KafkaClient/ClassPath": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return(
					detectortest.CmdlineArgsFromFile(t, filepath.Join("testdata", "kafka_client_cmdline")), nil)
			},
			want: &detector.Metadata{
				Categories: []detector.Category{detector.CategoryKafkaClient},
			},
		},
		"Process/KafkaClient/LoadedJars": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"java", "test.jar"}, nil)
				mp.On("OpenFilesWithContext", ctx).Return([]detector.OpenFilesStat{
					{Path: "test.jar"},
					{Path: "config/client.properties"},
					{Path: "kafka-metadata.jar"},
					{Path: "kafka-clients.jar"},
				}, nil)
			},
			want: &detector.Metadata{
				Categories: []detector.Category{detector.CategoryKafkaClient},
			},
		},
		"Process/KafkaClient/LoadedJarsDeleted": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"java", "test.jar"}, nil)
				mp.On("OpenFilesWithContext", ctx).Return([]detector.OpenFilesStat{
					{Path: "test.jar"},
					{Path: "config/client.properties"},
					{Path: "kafka-metadata.jar"},
					{Path: "kafka-clients.jar (deleted)"},
				}, nil)
			},
			want: &detector.Metadata{
				Categories: []detector.Category{detector.CategoryKafkaClient},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mp := new(detectortest.MockProcess)
			testCase.setup(mp)

			d := NewDetector(slog.Default())
			got, err := d.Detect(ctx, mp)
			if testCase.wantErr != nil {
				assert.ErrorIs(t, err, testCase.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCase.want, got)
			}
			mp.AssertExpectations(t)
		})
	}
}
