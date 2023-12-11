// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"testing"

	"github.com/go-test/deep"
	"github.com/influxdata/wlog"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/aws/amazon-cloudwatch-agent/logger"
)

func Test_getCollectorParams(t *testing.T) {
	type args struct {
		factories otelcol.Factories
		provider  otelcol.ConfigProvider
	}
	tests := []struct {
		name string
		args args
		want otelcol.CollectorSettings
	}{
		{
			name: "BuildInfoIsSet",
			args: args{
				factories: otelcol.Factories{},
				provider:  nil,
			},
			want: otelcol.CollectorSettings{
				Factories: func() (otelcol.Factories, error) {
					return otelcol.Factories{}, nil
				},
				ConfigProvider: nil,
				BuildInfo: component.BuildInfo{
					Command:     "CWAgent",
					Description: "CloudWatch Agent",
					Version:     "Unknown",
				},
				LoggingOptions: logger.NewLoggerOptions(os.Stderr, zap.NewAtomicLevelAt(zapcore.InfoLevel)),
			},
		},
	}
	for _, tt := range tests {
		logger.SetLevel(zap.NewAtomicLevelAt(zapcore.InfoLevel))
		wlog.SetLevel(wlog.INFO)
		t.Run(tt.name, func(t *testing.T) {
			got := getCollectorParams(tt.args.factories, tt.args.provider, os.Stderr)
			if deep.Equal(got, tt.want) != nil {
				t.Errorf("getCollectorParams() = %v, want %v", got, tt.want)
			}
		})
	}
}
