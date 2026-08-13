// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/influxdata/wlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/aws/amazon-cloudwatch-agent/logger"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
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

func TestFallbackOtelConfig(t *testing.T) {
	defaultYamlRelativePath := filepath.Join("default", paths.YAML)
	testCases := map[string]struct {
		tomlRelativePath string
		filesToCreate    []string
		want             string
	}{
		"WithoutAnyFiles": {
			tomlRelativePath: filepath.Join("config", "config.toml"),
			want:             defaultYamlRelativePath,
		},
		"WithDefaultYamlPath": {
			tomlRelativePath: filepath.Join("config", "config.toml"),
			filesToCreate:    []string{defaultYamlRelativePath, filepath.Join("config", paths.YAML)},
			want:             defaultYamlRelativePath,
		},
		"WithDefaultYamlInTomlDir": {
			tomlRelativePath: filepath.Join("config", "config.toml"),
			filesToCreate:    []string{filepath.Join("config", paths.YAML), filepath.Join("config", "config.yaml")},
			want:             filepath.Join("config", paths.YAML),
		},
		"WithSameNameAsToml": {
			tomlRelativePath: filepath.Join("config", "config.toml"),
			filesToCreate:    []string{filepath.Join("config", "config.yaml")},
			want:             filepath.Join("config", "config.yaml"),
		},
		"WithoutTomlPath": {
			tomlRelativePath: "",
			filesToCreate:    []string{filepath.Join("config", "config.yaml")},
			want:             defaultYamlRelativePath,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			for _, fileToCreate := range testCase.filesToCreate {
				path := filepath.Join(tmpDir, fileToCreate)
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
				require.NoError(t, os.WriteFile(path, nil, 0600))
			}
			got := getFallbackOtelConfig(filepath.Join(tmpDir, testCase.tomlRelativePath), filepath.Join(tmpDir, defaultYamlRelativePath))
			assert.Equal(t, filepath.Join(tmpDir, testCase.want), got)
		})
	}
}

func TestStopWaitsForDoneChannel(t *testing.T) {
	// Setup: initialize the global channels as run() would
	stop = make(chan struct{})
	done = make(chan struct{})

	p := &program{}

	// Simulate reloadLoop completing normally in background
	go func() {
		<-stop        // Wait for stop signal
		close(done)   // Signal completion (like defer close(done) in reloadLoop)
	}()

	// Call Stop() and verify it returns quickly (not timing out)
	start := time.Now()
	err := p.Stop(nil)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, elapsed, 1*time.Second, "Stop() should return quickly when done channel closes")
}
