// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-test/deep"
	"github.com/influxdata/wlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/merge/confmap"
	"github.com/aws/amazon-cloudwatch-agent/logger"
)

func Test_getCollectorParams(t *testing.T) {
	type args struct {
		factories        otelcol.Factories
		providerSettings otelcol.ConfigProviderSettings
	}

	_, loggerOptions := logger.NewLogger(os.Stderr, zap.NewAtomicLevelAt(zapcore.InfoLevel))
	tests := []struct {
		name string
		args args
		want otelcol.CollectorSettings
	}{
		{
			name: "BuildInfoIsSet",
			args: args{
				factories:        otelcol.Factories{},
				providerSettings: otelcol.ConfigProviderSettings{},
			},
			want: otelcol.CollectorSettings{
				Factories: func() (otelcol.Factories, error) {
					return otelcol.Factories{}, nil
				},
				ConfigProviderSettings: otelcol.ConfigProviderSettings{},
				BuildInfo: component.BuildInfo{
					Command:     "CWAgent",
					Description: "CloudWatch Agent",
					Version:     "Unknown",
				},
				LoggingOptions: loggerOptions,
			},
		},
	}
	for _, tt := range tests {
		logger.SetLevel(zap.NewAtomicLevelAt(zapcore.InfoLevel))
		wlog.SetLevel(wlog.INFO)
		t.Run(tt.name, func(t *testing.T) {
			got := getCollectorParams(tt.args.factories, tt.args.providerSettings, tt.want.LoggingOptions)
			if deep.Equal(got, tt.want) != nil {
				t.Errorf("getCollectorParams() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeConfigs(t *testing.T) {
	testEnvValue := `receivers:
  nop/1:
exporters:
  nop:
extensions:
  nop:
service:
  extensions: [nop]
  pipelines:
    traces:
      receivers: [nop/1]
      exporters: [nop]
`
	testCases := map[string]struct {
		input       []string
		isContainer bool
		envValue    string
		want        *confmap.Conf
		wantErr     bool
	}{
		"WithInvalidFile": {
			input:   []string{filepath.Join("not", "a", "file"), filepath.Join("testdata", "base.yaml")},
			wantErr: true,
		},
		"WithNoMerge": {
			input:   []string{filepath.Join("testdata", "base.yaml")},
			wantErr: false,
		},
		"WithoutEnv/Container": {
			input:       []string{filepath.Join("testdata", "base.yaml"), filepath.Join("testdata", "merge.yaml")},
			isContainer: true,
			want:        mustLoadFromFile(t, filepath.Join("testdata", "base+merge.yaml")),
		},
		"WithEnv/NonContainer": {
			input:       []string{filepath.Join("testdata", "base.yaml"), filepath.Join("testdata", "merge.yaml")},
			isContainer: false,
			envValue:    testEnvValue,
			want:        mustLoadFromFile(t, filepath.Join("testdata", "base+merge.yaml")),
		},
		"WithEnv/Container": {
			input:       []string{filepath.Join("testdata", "base.yaml")},
			isContainer: true,
			envValue:    testEnvValue,
			want:        mustLoadFromFile(t, filepath.Join("testdata", "base+env.yaml")),
		},
		"WithEmptyEnv/Container": {
			input:       []string{filepath.Join("testdata", "base.yaml")},
			isContainer: true,
			envValue:    "",
			want:        nil,
			wantErr:     false,
		},
		"WithInvalidEnv/Container": {
			input:       []string{filepath.Join("testdata", "base.yaml")},
			isContainer: true,
			envValue:    "test",
			wantErr:     true,
		},
		"WithIgnoredEnv/Container": {
			input:       []string{filepath.Join("testdata", "base.yaml"), filepath.Join("testdata", "merge.yaml")},
			isContainer: true,
			envValue:    testEnvValue,
			want:        mustLoadFromFile(t, filepath.Join("testdata", "base+merge.yaml")),
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			if testCase.isContainer {
				t.Setenv(envconfig.RunInContainer, envconfig.TrueValue)
			}
			t.Setenv(envconfig.CWOtelConfigContent, testCase.envValue)
			got, err := mergeConfigs(testCase.input)
			if testCase.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else if testCase.want == nil {
				assert.NoError(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, testCase.want.ToStringMap(), got.ToStringMap())
			}
		})
	}
}

func mustLoadFromFile(t *testing.T, path string) *confmap.Conf {
	conf, err := confmap.NewFileLoader(path).Load()
	require.NoError(t, err)
	return conf
}
