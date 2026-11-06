// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
)

// End-to-end: a config written via ConfigToYamlFile (which strips the null
// batch) resolves with the profiles queue bounded and the batcher off.
func TestProfilesExporterSendingQueueResolved(t *testing.T) {
	// Golden is raw translator output with the null batch; ConfigToYamlFile strips it.
	cfg := loadResolvedOtlpGolden(t)

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
