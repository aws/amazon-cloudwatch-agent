// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlphttp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
)

func TestTranslatorID(t *testing.T) {
	tr := NewTranslatorWithName("test", EndpointConfig{})
	assert.Equal(t, "otlp_http/test", tr.ID().String())
}

func TestTranslatorWithEndpoints(t *testing.T) {
	endpoint := EndpointConfig{
		LogsEndpoint:    "https://logs.us-west-2.amazonaws.com/v1/logs",
		MetricsEndpoint: "https://monitoring.us-west-2.amazonaws.com/v1/metrics",
		TracesEndpoint:  "https://xray.us-west-2.amazonaws.com/v1/traces",
	}
	tr := NewTranslatorWithName("full", endpoint)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	otlpCfg := cfg.(*otlphttpexporter.Config)
	assert.Equal(t, "https://logs.us-west-2.amazonaws.com/v1/logs", otlpCfg.LogsEndpoint)
	assert.Equal(t, "https://monitoring.us-west-2.amazonaws.com/v1/metrics", otlpCfg.MetricsEndpoint)
	assert.Equal(t, "https://xray.us-west-2.amazonaws.com/v1/traces", otlpCfg.TracesEndpoint)
}

func TestTranslatorWithAuthenticator(t *testing.T) {
	authID := component.NewIDWithName(component.MustNewType("sigv4auth"), "test")
	tr := NewTranslatorWithName("auth_test", EndpointConfig{},
		WithAuthenticator(authID),
	)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	otlpCfg := cfg.(*otlphttpexporter.Config)
	require.True(t, otlpCfg.ClientConfig.Auth.HasValue())
	assert.Equal(t, authID, otlpCfg.ClientConfig.Auth.Get().AuthenticatorID)
}

func TestTranslatorWithSendingQueueBatchMetadataKeys(t *testing.T) {
	keys := []string{"aws.log.group.name", "aws.log.stream.name"}
	tr := NewTranslatorWithName("logs", EndpointConfig{LogsEndpoint: "https://logs.us-west-2.amazonaws.com/v1/logs"},
		WithSendingQueueBatchMetadataKeys(keys...),
	)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	otlpCfg := cfg.(*otlphttpexporter.Config)
	require.True(t, otlpCfg.QueueConfig.HasValue())
	qc := otlpCfg.QueueConfig.Get()
	require.True(t, qc.Batch.HasValue(), "setting partition metadata keys should activate the exporter batcher with a partitioner")
	assert.Equal(t, keys, qc.Batch.Get().Partition.MetadataKeys)
}

func TestTranslatorDefaultHasNoBatchPartitionKeys(t *testing.T) {
	tr := NewTranslatorWithName("logs", EndpointConfig{LogsEndpoint: "https://logs.us-west-2.amazonaws.com/v1/logs"})

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	otlpCfg := cfg.(*otlphttpexporter.Config)
	require.True(t, otlpCfg.QueueConfig.HasValue())
	// Without the option the translator leaves the factory default untouched: Batch stays the
	// "default" configoptional flavor (HasValue()==false) so no partition keys are set here.
	assert.False(t, otlpCfg.QueueConfig.Get().Batch.HasValue())
}

func TestTranslatorWithEmptyMetadataKeysIsNoop(t *testing.T) {
	// Mirrors the AppSignals fully-static logs case (no from_context routing): passing no keys
	// must not touch the exporter batcher.
	tr := NewTranslatorWithName("logs", EndpointConfig{LogsEndpoint: "https://logs.us-west-2.amazonaws.com/v1/logs"},
		WithSendingQueueBatchMetadataKeys(),
	)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	otlpCfg := cfg.(*otlphttpexporter.Config)
	require.True(t, otlpCfg.QueueConfig.HasValue())
	assert.False(t, otlpCfg.QueueConfig.Get().Batch.HasValue())
}
