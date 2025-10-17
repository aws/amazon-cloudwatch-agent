// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package extract

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/internal/detector"
	"github.com/aws/amazon-cloudwatch-agent/internal/detector/detectortest"
)

func TestNameExtractor(t *testing.T) {
	ctx := context.Background()
	tests := map[string]struct {
		setup   func(*detectortest.MockProcess)
		want    string
		wantErr error
	}{
		"Process/Error": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return(nil, assert.AnError)
				mp.On("EnvironWithContext", ctx).Return(nil, assert.AnError)
			},
			wantErr: detector.ErrIncompatibleExtractor,
		},
		"Process/NotTomcat": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{"java"}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{"KEY=VALUE"}, nil)
			},
			wantErr: detector.ErrIncompatibleExtractor,
		},
		"CatalinaBase/SystemProperty": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{
					"java",
					"-Dcatalina.base=/opt/tomcat/instance-1",
					"org.apache.catalina.startup.Bootstrap",
					"start",
				}, nil)
			},
			want: "/opt/tomcat/instance-1",
		},
		"CatalinaBase/Env": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{
					"java",
					"org.apache.catalina.startup.Bootstrap",
					"start",
				}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{
					"CATALINA_BASE=/opt/tomcat/instance-2",
				}, nil)
			},
			want: "/opt/tomcat/instance-2",
		},
		"CatalinaHome/SystemProperty": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{
					"java",
					"-Dcatalina.base=",
					`-Dcatalina.home="/opt/tomcat/home-1"`,
					"org.apache.catalina.startup.Bootstrap",
					"start",
				}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{
					"CATALINA_HOME=/opt/tomcat/home-2",
				}, nil)
			},
			want: "/opt/tomcat/home-1",
		},
		"CatalinaHome/Env": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{
					"java",
					"org.apache.catalina.startup.Bootstrap",
					"start",
				}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{
					"OTHER_KEY",
					"CATALINA_BASE=",
					"CATALINA_HOME=/opt/tomcat/home-2",
				}, nil)
			},
			want: "/opt/tomcat/home-2",
		},
		"PreferCatalinaBaseWhenBothPresent/SystemProperty": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{
					"java",
					"-Dcatalina.base=/opt/tomcat/instance-1",
					"-Dcatalina.home=/opt/tomcat/home-1",
					"org.apache.catalina.startup.Bootstrap",
					"start",
				}, nil)
			},
			want: "/opt/tomcat/instance-1",
		},
		"PreferCatalinaBaseWhenBothPresent/Env": {
			setup: func(mp *detectortest.MockProcess) {
				mp.On("CmdlineSliceWithContext", ctx).Return([]string{
					"java",
					"-Dcatalina.home=/opt/tomcat/home-1",
					"org.apache.catalina.startup.Bootstrap",
					"start",
				}, nil)
				mp.On("EnvironWithContext", ctx).Return([]string{
					"CATALINA_HOME=/opt/tomcat/home-2",
					"CATALINA_BASE=/opt/tomcat/instance-2",
				}, nil)
			},
			want: "/opt/tomcat/instance-2",
		},
	}
	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			mp := new(detectortest.MockProcess)
			testCase.setup(mp)

			extractor := NewNameExtractor(slog.Default())
			got, err := extractor.Extract(ctx, mp)
			if testCase.wantErr != nil {
				assert.ErrorIs(t, err, testCase.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, testCase.want, got)
			mp.AssertExpectations(t)
		})
	}
}
