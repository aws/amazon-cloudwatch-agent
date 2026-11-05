// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-cloudwatch-agent/service/defaultcomponents"
	"github.com/aws/amazon-cloudwatch-agent/translator/cmdutil"
)

// End-to-end: a config written via ConfigToYamlFile (which strips the null
// batch) resolves with the profiles queue bounded and the batcher off.
func TestProfilesExporterSendingQueueResolved(t *testing.T) {
	factories, err := defaultcomponents.Factories()
	require.NoError(t, err)

	// Golden is raw translator output with the null batch; ConfigToYamlFile strips it.
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

	// Profiles exporter: bounded queue, batcher off.
	profilesID := component.MustNewIDWithName("otlp_http", "profiles")
	profiles, ok := cfg.Exporters[profilesID].(*otlphttpexporter.Config)
	require.True(t, ok, "profiles exporter missing or wrong type")
	require.True(t, profiles.QueueConfig.HasValue(), "profiles exporter should have an explicit sending_queue")
	profilesQueue := profiles.QueueConfig.Get()
	assert.EqualValues(t, 100, profilesQueue.QueueSize)
	assert.Equal(t, "requests", profilesQueue.Sizer.String())
	assert.False(t, profilesQueue.BlockOnOverflow, "queue must stay non-blocking")
	assert.False(t, profilesQueue.Batch.HasValue(), "exporter batcher must stay off (null batch stripped on write)")
	assert.Nil(t, profilesQueue.StorageID, "queue must not use persistent storage")
	assert.NoError(t, profiles.Validate())

	// Other signals keep the default queue.
	for _, name := range []string{"metrics", "logs", "traces"} {
		id := component.MustNewIDWithName("otlp_http", name)
		other, ok := cfg.Exporters[id].(*otlphttpexporter.Config)
		require.True(t, ok, "%s exporter missing or wrong type", name)
		require.True(t, other.QueueConfig.HasValue(), "%s exporter should keep the default sending_queue", name)
		assert.EqualValues(t, 1000, other.QueueConfig.Get().QueueSize, "%s queue should keep the factory default", name)
		assert.False(t, other.QueueConfig.Get().Batch.HasValue(), "%s batcher must stay off", name)
	}
}
