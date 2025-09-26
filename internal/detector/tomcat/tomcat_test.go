// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tomcat

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/detectortest"
)

func TestTomcatDetector_Mock(t *testing.T) {
	type mocks struct {
		process       *detectortest.MockProcess
		nameExtractor *detectortest.MockExtractor[string]
	}

	ctx := context.Background()
	testCases := map[string]struct {
		setup   func(*mocks)
		want    *detector.Metadata
		wantErr error
	}{
		"Success": {
			setup: func(m *mocks) {
				m.nameExtractor.On("Extract", ctx, m.process).Return("/opt/tomcat/latest", nil)
			},
			want: &detector.Metadata{
				Categories: []detector.Category{detector.CategoryTomcat},
				Name:       "/opt/tomcat/latest",
			},
		},
		"NameExtractor/Error": {
			setup: func(m *mocks) {
				m.nameExtractor.On("Extract", ctx, m.process).Return("", assert.AnError)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			m := &mocks{
				process:       new(detectortest.MockProcess),
				nameExtractor: new(detectortest.MockExtractor[string]),
			}
			testCase.setup(m)

			d := NewDetector(slog.Default())
			td, ok := d.(*tomcatDetector)
			require.True(t, ok)
			td.nameExtractor = m.nameExtractor
			got, err := d.Detect(ctx, m.process)
			if testCase.wantErr != nil {
				assert.ErrorIs(t, err, testCase.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCase.want, got)
			}
			m.process.AssertExpectations(t)
			m.nameExtractor.AssertExpectations(t)
		})
	}
}

func TestTomcatDetector_Actual(t *testing.T) {
	ctx := context.Background()
	testCases := map[string]struct {
		setup   func(*detectortest.MockProcess)
		want    *detector.Metadata
		wantErr error
	}{
		"Process/Error": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return(nil, assert.AnError)
				mp.On("EnvironWithContext", ctx).Return(nil, assert.AnError)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
		"Process/NotTomcat": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"java"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{}, nil)
			},
			wantErr: detector.ErrIncompatibleDetector,
		},
		"Process/Tomcat": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{
					"java",
					"-Djava.util.logging.config.file=/opt/tomcat/latest/conf/logging.properties",
					"-Djava.util.logging.manager=org.apache.juli.ClassLoaderLogManager",
					"-Djdk.tls.ephemeralDHKeySize=2048",
					"-Djava.protocol.handler.pkgs=org.apache.catalina.webresources",
					"-Dsun.io.useCanonCaches=false",
					"-Dorg.apache.catalina.security.SecurityListener.UMASK=0027",
					"-Dcom.sun.management.jmxremote",
					"-Dcom.sun.management.jmxremote.port=1080",
					"-Dcom.sun.management.jmxremote.ssl=false",
					"-Dcom.sun.management.jmxremote.authenticate=false",
					"-Dignore.endorsed.dirs=",
					"-classpath",
					"/opt/tomcat/latest/bin/bootstrap.jar:/opt/tomcat/latest/bin/tomcat-juli.jar",
					"-Dcatalina.base=/opt/tomcat/latest",
					"-Dcatalina.home=/opt/tomcat/latest",
					"-Djava.io.tmpdir=/opt/tomcat/latest/temp",
					"org.apache.catalina.startup.Bootstrap",
					"start",
				}, nil)
			},
			want: &detector.Metadata{
				Categories: []detector.Category{detector.CategoryTomcat},
				Name:       "/opt/tomcat/latest",
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
