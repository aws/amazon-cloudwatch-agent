// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/service/defaultcomponents"
)

// TestProfilesExporterSendingQueueResolved loads a translated golden through the
// real config provider and component factories and asserts the resolved otlphttp
// exporter configs. This is the end-to-end counterpart to the golden-translation
// tests: it proves the emitted sending_queue reloads into the config the
// translator intends (bounded queue, batcher off), rather than hand-building the
// serialized input. It also guards that the other signals keep the factory
// default queue, i.e. the profiles cap does not leak onto metrics/logs/traces.
func TestProfilesExporterSendingQueueResolved(t *testing.T) {
	factories, err := defaultcomponents.Factories()
	require.NoError(t, err)

	providerSettings := GetSettings(
		[]string{filepath.Join("../../translator/tocwconfig/sampleConfig/opentelemetry", "otlp_otel_config.yaml")},
		zap.NewNop(),
	)
	provider, err := otelcol.NewConfigProvider(providerSettings)
	require.NoError(t, err)
	cfg, err := provider.Get(context.Background(), factories)
	require.NoError(t, err)

	// Profiles exporter: explicit bounded queue, batcher off.
	profilesID := component.MustNewIDWithName("otlp_http", "profiles")
	profiles, ok := cfg.Exporters[profilesID].(*otlphttpexporter.Config)
	require.True(t, ok, "profiles exporter missing or wrong type")
	require.True(t, profiles.QueueConfig.HasValue(), "profiles exporter should have an explicit sending_queue")
	profilesQueue := profiles.QueueConfig.Get()
	assert.EqualValues(t, 64, profilesQueue.QueueSize)
	assert.Equal(t, "requests", profilesQueue.Sizer.String())
	assert.False(t, profilesQueue.BlockOnOverflow, "queue must stay non-blocking")
	assert.False(t, profilesQueue.Batch.HasValue(), "exporter batcher must stay off (null batch key stripped at emission)")
	assert.Nil(t, profilesQueue.StorageID, "queue must not use persistent storage")
	assert.NoError(t, profiles.Validate())

	// Other signals keep the factory default queue (1000 requests), unaffected
	// by the profiles cap.
	for _, name := range []string{"metrics", "logs", "traces"} {
		id := component.MustNewIDWithName("otlp_http", name)
		other, ok := cfg.Exporters[id].(*otlphttpexporter.Config)
		require.True(t, ok, "%s exporter missing or wrong type", name)
		require.True(t, other.QueueConfig.HasValue(), "%s exporter should keep the default sending_queue", name)
		assert.EqualValues(t, 1000, other.QueueConfig.Get().QueueSize, "%s queue should keep the factory default", name)
		assert.False(t, other.QueueConfig.Get().Batch.HasValue(), "%s batcher must stay off", name)
	}
}
