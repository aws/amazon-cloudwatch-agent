// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/service/defaultcomponents"
	"github.com/aws/amazon-cloudwatch-agent/translator/cmdutil"
)

func loadResolvedOtlpGolden(t *testing.T) *otelcol.Config {
	t.Helper()
	factories, err := defaultcomponents.Factories()
	require.NoError(t, err)

	golden, err := os.ReadFile(filepath.Join("../../translator/tocwconfig/sampleConfig/opentelemetry", "otlp_otel_config.yaml"))
	require.NoError(t, err)
	var cfgMap map[string]any
	require.NoError(t, yaml.Unmarshal(golden, &cfgMap))

	yamlPath := filepath.Join(t.TempDir(), "otlp_otel_config.yaml")
	require.NoError(t, cmdutil.ConfigToYamlFile(cfgMap, yamlPath))

	providerSettings := GetSettings([]string{yamlPath}, zap.NewNop())
	provider, err := otelcol.NewConfigProvider(providerSettings)
	require.NoError(t, err)
	cfg, err := provider.Get(context.Background(), factories)
	require.NoError(t, err)
	return cfg
}

func TestProfilesVersionHeaderResolved(t *testing.T) {
	cfg := loadResolvedOtlpGolden(t)

	headersID := component.MustNewIDWithName("headers_setter", "profiles")
	headers, ok := cfg.Extensions[headersID].(*headerssetterextension.Config)
	require.True(t, ok, "profiles headers_setter extension missing or wrong type")
	require.Len(t, headers.HeadersConfig, 1)
	require.NotNil(t, headers.HeadersConfig[0].Value)
	assert.Equal(t, "1", *headers.HeadersConfig[0].Value)
	assert.Equal(t, "OTLP-Profiles-Development-Version", *headers.HeadersConfig[0].Key)
	assert.Equal(t, headerssetterextension.INSERT, headers.HeadersConfig[0].Action)
	require.NotNil(t, headers.AdditionalAuth)
	assert.Equal(t, component.MustNewIDWithName("sigv4auth", "monitoring"), *headers.AdditionalAuth)
	assert.Contains(t, cfg.Service.Extensions, headersID)
}
