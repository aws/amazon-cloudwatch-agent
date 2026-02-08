// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package kafkabroker

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

func TestKafkaDetector_Mock(t *testing.T) {
	type mocks struct {
		process             *detectortest.MockProcess
		attributesExtractor *detectortest.MockExtractor[map[string]string]
	}

	ctx := context.Background()
	testCases := map[string]struct {
		setup   func(*mocks)
		want    *detector.Metadata
		wantErr error
	}{
		"Success/Attributes": {
			setup: func(m *mocks) {
				m.attributesExtractor.On("Extract", ctx, m.process).Return(map[string]string{
					"cluster.id": "1234",
				}, nil)
			},
			want: &detector.Metadata{
				Categories: []detector.Category{detector.CategoryKafkaBroker},
				Name:       brokerMetadataName,
				Attributes: map[string]string{
					"cluster.id": "1234",
				},
			},
		},
		"Success/NoAttributes": {
			setup: func(m *mocks) {
				m.attributesExtractor.On("Extract", ctx, m.process).Return(map[string]string{}, nil)
			},
			want: &detector.Metadata{
				Categories: []detector.Category{detector.CategoryKafkaBroker},
				Name:       brokerMetadataName,
			},
		},
		"AttributesExtractor/Error": {
			setup: func(m *mocks) {
				m.attributesExtractor.On("Extract", ctx, m.process).Return(map[string]string{}, assert.AnError)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			m := &mocks{
				process:             new(detectortest.MockProcess),
				attributesExtractor: new(detectortest.MockExtractor[map[string]string]),
			}
			testCase.setup(m)

			d := NewDetector(slog.Default())
			td, ok := d.(*kafkaBrokerDetector)
			require.True(t, ok)
			td.attributesExtractor = m.attributesExtractor
			got, err := d.Detect(ctx, m.process)
			if testCase.wantErr != nil {
				assert.ErrorIs(t, err, testCase.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCase.want, got)
			}
			m.process.AssertExpectations(t)
			m.attributesExtractor.AssertExpectations(t)
		})
	}
}

func TestKafkaDetector_Actual(t *testing.T) {
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
		"Process/NotKafkaBroker": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"java"}, nil)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
		"Process/KafkaBroker": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return(
					detectortest.CmdlineArgsFromFile(t, filepath.Join("testdata", "kafka_broker_cmdline")), nil)
				mp.On("CwdWithContext", ctx).Return(filepath.Join("testdata"), nil)
			},
			want: &detector.Metadata{
				Categories: []detector.Category{detector.CategoryKafkaBroker},
				Name:       "Kafka Broker",
				Attributes: map[string]string{
					"cluster.id": "WQSzAfd_RvO0TocjqhQoaA",
					"broker.id":  "0",
				},
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
