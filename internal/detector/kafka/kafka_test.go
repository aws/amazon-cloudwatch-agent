// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package kafka

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

func TestKafkaDetector(t *testing.T) {
	ctx := context.Background()
	testCases := map[string]struct {
		setup   func(*detectortest.MockProcess)
		want    *detector.Metadata
		wantErr error
	}{
		"Process/Error": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return(nil, assert.AnError)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
		"Process/NotKafka": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"/usr/bin/python"}, nil)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
		"Process/NotKafkaBroker": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return(
					detectortest.CmdlineArgsFromFile(t, filepath.Join("testdata", "zookeeper_cmdline")), nil)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
		"Process/KafkaBroker/SimpleCmdline": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{
					"java",
					"kafka.Kafka",
					"config/server.properties",
				}, nil)
			},
			want: &detector.Metadata{
				Categories: []detector.Category{detector.CategoryKafkaBroker},
				Name:       "Kafka Broker",
			},
		},
		"Process/KafkaBroker/ComplexCmdline": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return(
					detectortest.CmdlineArgsFromFile(t, filepath.Join("testdata", "kafka_broker_cmdline")), nil)
			},
			want: &detector.Metadata{
				Categories: []detector.Category{detector.CategoryKafkaBroker},
				Name:       "Kafka Broker",
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
