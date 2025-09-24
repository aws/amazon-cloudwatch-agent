// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package java

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/detectortest"
)

func TestJavaDetector(t *testing.T) {
	type mocks struct {
		process       *detectortest.MockProcess
		subDetector   *detectortest.MockProcessDetector
		nameExtractor *detectortest.MockExtractor[string]
		portExtractor *detectortest.MockExtractor[int]
	}

	ctx := context.Background()
	testCases := map[string]struct {
		setup   func(m *mocks)
		want    *detector.Metadata
		wantErr error
	}{
		"Process/Error": {
			setup: func(m *mocks) {
				m.process.On("ExeWithContext", ctx).Return("", assert.AnError)
			},
			wantErr: assert.AnError,
		},
		"Process/NotJava": {
			setup: func(m *mocks) {
				m.process.On("ExeWithContext", ctx).Return("/usr/bin/python", nil)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
		"SubDetector/Success": {
			setup: func(m *mocks) {
				m.process.On("ExeWithContext", ctx).Return("/usr/bin/java", nil)
				m.subDetector.On("Detect", ctx, m.process).Return(&detector.Metadata{
					Categories: []detector.Category{detector.CategoryJVM, detector.CategoryTomcat},
					Name:       "tomcat",
					Status:     detector.StatusReady,
				}, nil)
			},
			want: &detector.Metadata{
				Categories: []detector.Category{detector.CategoryJVM, detector.CategoryTomcat},
				Name:       "tomcat",
				Status:     detector.StatusReady,
			},
		},
		"SubDetector/FallbackToJava": {
			setup: func(m *mocks) {
				m.process.On("ExeWithContext", ctx).Return("/usr/bin/java", nil)
				m.subDetector.On("Detect", ctx, m.process).Return(nil, detector.ErrIncompatibleDetector)
				m.nameExtractor.On("Extract", ctx, m.process).Return("my-application", nil)
				m.portExtractor.On("Extract", ctx, m.process).Return(1234, nil)
			},
			want: &detector.Metadata{
				Categories:    []detector.Category{detector.CategoryJVM},
				Name:          "my-application",
				Status:        detector.StatusReady,
				TelemetryPort: 1234,
			},
		},
		"NameExtractor/Error": {
			setup: func(m *mocks) {
				m.process.On("ExeWithContext", ctx).Return("/usr/bin/java", nil)
				m.subDetector.On("Detect", ctx, m.process).Return(nil, detector.ErrIncompatibleDetector)
				m.nameExtractor.On("Extract", ctx, m.process).Return("", detector.ErrSkipProcess)
			},
			wantErr: detector.ErrSkipProcess,
		},
		"VersionExtractor/Error": {
			setup: func(m *mocks) {
				m.process.On("ExeWithContext", ctx).Return("/usr/bin/java", nil)
				m.subDetector.On("Detect", ctx, m.process).Return(nil, detector.ErrIncompatibleDetector)
				m.nameExtractor.On("Extract", ctx, m.process).Return("my-application", nil)
				m.portExtractor.On("Extract", ctx, m.process).Return(1234, nil)
			},
			want: &detector.Metadata{
				Categories:    []detector.Category{detector.CategoryJVM},
				Name:          "my-application",
				Status:        detector.StatusReady,
				TelemetryPort: 1234,
			},
		},
		"PortExtractor/Error": {
			setup: func(m *mocks) {
				m.process.On("ExeWithContext", ctx).Return("/usr/bin/java", nil)
				m.subDetector.On("Detect", ctx, m.process).Return(nil, detector.ErrIncompatibleDetector)
				m.nameExtractor.On("Extract", ctx, m.process).Return("my-application", nil)
				m.portExtractor.On("Extract", ctx, m.process).Return(-1, detector.ErrIncompatibleExtractor)
			},
			want: &detector.Metadata{
				Categories: []detector.Category{detector.CategoryJVM},
				Name:       "my-application",
				Status:     detector.StatusNeedsSetupJmxPort,
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			m := &mocks{
				process:       new(detectortest.MockProcess),
				subDetector:   new(detectortest.MockProcessDetector),
				nameExtractor: new(detectortest.MockExtractor[string]),
				portExtractor: new(detectortest.MockExtractor[int]),
			}
			testCase.setup(m)

			d := NewDetector(slog.Default())
			jd, ok := d.(*javaDetector)
			require.True(t, ok)
			jd.subDetectors = []detector.ProcessDetector{m.subDetector}
			jd.nameExtractor = m.nameExtractor
			jd.portExtractor = m.portExtractor

			got, err := d.Detect(ctx, m.process)
			if testCase.wantErr != nil {
				assert.ErrorIs(t, err, testCase.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCase.want, got)
			}
			m.process.AssertExpectations(t)
			m.subDetector.AssertExpectations(t)
			m.nameExtractor.AssertExpectations(t)
			m.portExtractor.AssertExpectations(t)
		})
	}
}
