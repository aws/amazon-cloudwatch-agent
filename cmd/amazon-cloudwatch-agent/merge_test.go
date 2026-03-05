// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/merge/confmap"
)

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
    traces/1:
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
			input:   []string{filepath.Join("testdata", "invalid.yaml"), filepath.Join("testdata", "base.yaml")},
			wantErr: true,
		},
		"WithAllMissingFiles": {
			input: []string{filepath.Join("not", "a", "file"), filepath.Join("also", "not", "a", "file")},
			want:  nil,
		},
		"WithMissingFile": {
			input: []string{filepath.Join("not", "a", "file"), filepath.Join("testdata", "base.yaml")},
			want:  mustLoadFromFile(t, filepath.Join("testdata", "base.yaml")),
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
		"WithEnv/Container/MultipleFiles": {
			input:       []string{filepath.Join("testdata", "base.yaml"), filepath.Join("testdata", "merge.yaml")},
			isContainer: true,
			envValue:    testEnvValue,
			want:        mustLoadFromFile(t, filepath.Join("testdata", "base+merge+env.yaml")),
		},
		"WithAgentHealth": {
			input: []string{filepath.Join("testdata", "base.yaml"), filepath.Join("testdata", "awsemf.yaml")},
			want:  mustLoadFromFile(t, filepath.Join("testdata", "base+awsemf.yaml")),
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			if testCase.isContainer {
				t.Setenv(envconfig.RunInContainer, envconfig.TrueValue)
			}
			t.Setenv(envconfig.CWOtelConfigContent, testCase.envValue)
			got, err := mergeConfigs(testCase.input, true)
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

func TestMergeConfigs_UsageDataDisabled(t *testing.T) {
	got, err := mergeConfigs(
		[]string{filepath.Join("testdata", "base.yaml"), filepath.Join("testdata", "awsemf.yaml")},
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, got)
	assertNoExtensions(t, got.ToStringMap())
}

func TestMergeAgentHealth_NilConf(t *testing.T) {
	assert.Nil(t, mergeAgentHealth(nil, true))
}

func TestMergeAgentHealth_NoExporters(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"receivers": map[string]any{"otlp": map[string]any{}},
	})
	got := mergeAgentHealth(conf, true)
	assertNoExtensions(t, got.ToStringMap())
}

func TestMergeAgentHealth_NoAWSExporters(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"exporters": map[string]any{
			"debug": map[string]any{},
		},
	})
	got := mergeAgentHealth(conf, true)
	assertNoExtensions(t, got.ToStringMap())
}

func TestMergeAgentHealth_AWSSingleExporter(t *testing.T) {
	testCases := map[string]struct {
		exporterKey    string
		wantMiddleware string
		wantOperations []any
	}{
		"awsemf":              {exporterKey: "awsemf", wantMiddleware: "agenthealth/logs", wantOperations: []any{"PutLogEvents"}},
		"awsemf_named":        {exporterKey: "awsemf/custom", wantMiddleware: "agenthealth/logs", wantOperations: []any{"PutLogEvents"}},
		"awscloudwatchlogs":   {exporterKey: "awscloudwatchlogs", wantMiddleware: "agenthealth/logs", wantOperations: []any{"PutLogEvents"}},
		"awsxray":             {exporterKey: "awsxray", wantMiddleware: "agenthealth/traces", wantOperations: []any{"PutTraceSegments"}},
		"awsxray_named":       {exporterKey: "awsxray/custom", wantMiddleware: "agenthealth/traces", wantOperations: []any{"PutTraceSegments"}},
		"awscloudwatch":       {exporterKey: "awscloudwatch", wantMiddleware: "agenthealth/metrics", wantOperations: []any{"PutMetricData"}},
		"awscloudwatch_named": {exporterKey: "awscloudwatch/custom", wantMiddleware: "agenthealth/metrics", wantOperations: []any{"PutMetricData"}},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(map[string]any{
				"exporters": map[string]any{
					testCase.exporterKey: map[string]any{},
				},
			})
			got := mergeAgentHealth(conf, true).ToStringMap()

			// Check middleware set on exporter
			exporters := got["exporters"].(map[string]any)
			expCfg := exporters[testCase.exporterKey].(map[string]any)
			assert.Equal(t, testCase.wantMiddleware, expCfg["middleware"])

			// Check extension definition with stats
			extensions := getExtensions(t, got)
			extCfg := extensions[testCase.wantMiddleware].(map[string]any)
			assert.Equal(t, true, extCfg["is_usage_data_enabled"])
			stats := extCfg["stats"].(map[string]any)
			assert.Equal(t, testCase.wantOperations, stats["operations"])

			// Check service extensions list
			assert.Contains(t, getSvcExtensions(t, got), testCase.wantMiddleware)
		})
	}
}

func TestMergeAgentHealth_MultipleExporters(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"exporters": map[string]any{
			"awsemf":        map[string]any{},
			"awsxray":       map[string]any{},
			"awscloudwatch": map[string]any{},
			"debug":         map[string]any{},
		},
	})
	got := mergeAgentHealth(conf, true).ToStringMap()

	exporters := got["exporters"].(map[string]any)
	assert.Equal(t, "agenthealth/logs", exporters["awsemf"].(map[string]any)["middleware"])
	assert.Equal(t, "agenthealth/traces", exporters["awsxray"].(map[string]any)["middleware"])
	assert.Equal(t, "agenthealth/metrics", exporters["awscloudwatch"].(map[string]any)["middleware"])
	debugCfg := exporters["debug"].(map[string]any)
	_, hasMiddleware := debugCfg["middleware"]
	assert.False(t, hasMiddleware)

	svcExts := getSvcExtensions(t, got)
	assert.Contains(t, svcExts, "agenthealth/logs")
	assert.Contains(t, svcExts, "agenthealth/traces")
	assert.Contains(t, svcExts, "agenthealth/metrics")
}

func TestMergeAgentHealth_DoesNotOverwriteExistingMiddleware(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"exporters": map[string]any{
			"awsemf": map[string]any{
				"middleware": "custom/extension",
			},
		},
	})
	got := mergeAgentHealth(conf, true).ToStringMap()

	exporters := got["exporters"].(map[string]any)
	assert.Equal(t, "custom/extension", exporters["awsemf"].(map[string]any)["middleware"])
	assertNoExtensions(t, got)
}

func TestMergeAgentHealth_PartialCustomMiddleware(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"exporters": map[string]any{
			"awsemf":            map[string]any{"middleware": "custom/extension"},
			"awscloudwatchlogs": map[string]any{},
		},
	})
	got := mergeAgentHealth(conf, true).ToStringMap()

	exporters := got["exporters"].(map[string]any)
	assert.Equal(t, "custom/extension", exporters["awsemf"].(map[string]any)["middleware"])
	assert.Equal(t, "agenthealth/logs", exporters["awscloudwatchlogs"].(map[string]any)["middleware"])

	extensions := getExtensions(t, got)
	_, hasAgentHealth := extensions["agenthealth/logs"]
	assert.True(t, hasAgentHealth)
	assert.Equal(t, 1, count(getSvcExtensions(t, got), "agenthealth/logs"))
}

func TestMergeAgentHealth_PreservesExistingExtensions(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"exporters": map[string]any{
			"awsemf": map[string]any{},
		},
		"extensions": map[string]any{
			"health_check": map[string]any{},
		},
		"service": map[string]any{
			"extensions": []any{"health_check"},
		},
	})
	got := mergeAgentHealth(conf, true).ToStringMap()

	extensions := getExtensions(t, got)
	_, hasHealthCheck := extensions["health_check"]
	assert.True(t, hasHealthCheck)
	_, hasAgentHealth := extensions["agenthealth/logs"]
	assert.True(t, hasAgentHealth)

	svcExts := getSvcExtensions(t, got)
	assert.Contains(t, svcExts, "health_check")
	assert.Contains(t, svcExts, "agenthealth/logs")
}

func TestMergeAgentHealth_DoesNotDuplicateExtension(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"exporters": map[string]any{
			"awsemf":            map[string]any{},
			"awscloudwatchlogs": map[string]any{},
		},
	})
	got := mergeAgentHealth(conf, true).ToStringMap()

	assert.Equal(t, 1, count(getSvcExtensions(t, got), "agenthealth/logs"))
}

func TestMergeAgentHealth_DoesNotOverwriteExistingAgentHealth(t *testing.T) {
	existingCfg := map[string]any{"is_usage_data_enabled": false}
	conf := confmap.NewFromStringMap(map[string]any{
		"exporters": map[string]any{
			"awsemf": map[string]any{},
		},
		"extensions": map[string]any{
			"agenthealth/logs": existingCfg,
		},
		"service": map[string]any{
			"extensions": []any{"agenthealth/logs"},
		},
	})
	got := mergeAgentHealth(conf, true).ToStringMap()

	extensions := getExtensions(t, got)
	assert.Equal(t, existingCfg, extensions["agenthealth/logs"])

	assert.Equal(t, 1, count(getSvcExtensions(t, got), "agenthealth/logs"))
}

func getSvcExtensions(t *testing.T, m map[string]any) []any {
	t.Helper()
	svc, ok := m["service"].(map[string]any)
	require.True(t, ok, "expected service section in config map")
	exts, ok := svc["extensions"].([]any)
	require.True(t, ok, "expected extensions list in service section")
	return exts
}

func getExtensions(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	exts, ok := m["extensions"].(map[string]any)
	require.True(t, ok, "expected extensions section in config map")
	return exts
}

func assertNoExtensions(t *testing.T, m map[string]any) {
	t.Helper()
	_, hasExtensions := m["extensions"]
	assert.False(t, hasExtensions)
}

func count(slice []any, value any) int {
	n := 0
	for _, v := range slice {
		if v == value {
			n++
		}
	}
	return n
}

func mustLoadFromFile(t *testing.T, path string) *confmap.Conf {
	conf, err := confmap.NewFileLoader(path).Load()
	require.NoError(t, err)
	return conf
}
