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
		LogsEndpoint:     "https://logs.us-west-2.amazonaws.com/v1/logs",
		MetricsEndpoint:  "https://monitoring.us-west-2.amazonaws.com/v1/metrics",
		TracesEndpoint:   "https://xray.us-west-2.amazonaws.com/v1/traces",
		ProfilesEndpoint: "https://monitoring.us-west-2.amazonaws.com/v1development/profiles",
	}
	tr := NewTranslatorWithName("full", endpoint)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	otlpCfg := cfg.(*otlphttpexporter.Config)
	assert.Equal(t, "https://logs.us-west-2.amazonaws.com/v1/logs", otlpCfg.LogsEndpoint)
	assert.Equal(t, "https://monitoring.us-west-2.amazonaws.com/v1/metrics", otlpCfg.MetricsEndpoint)
	assert.Equal(t, "https://xray.us-west-2.amazonaws.com/v1/traces", otlpCfg.TracesEndpoint)
	assert.Equal(t, "https://monitoring.us-west-2.amazonaws.com/v1development/profiles", otlpCfg.ProfilesEndpoint)
}

func TestTranslatorWithProfilesEndpointOnly(t *testing.T) {
	tr := NewTranslatorWithName("profiles", EndpointConfig{
		ProfilesEndpoint: "https://monitoring.us-east-1.amazonaws.com/v1development/profiles",
	})

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	otlpCfg := cfg.(*otlphttpexporter.Config)
	assert.Equal(t, "https://monitoring.us-east-1.amazonaws.com/v1development/profiles", otlpCfg.ProfilesEndpoint)
	assert.Empty(t, otlpCfg.LogsEndpoint)
	assert.Empty(t, otlpCfg.MetricsEndpoint)
	assert.Empty(t, otlpCfg.TracesEndpoint)
	assert.NoError(t, otlpCfg.Validate())
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

func TestTranslatorDisablesQueueBatcher(t *testing.T) {
	// Outer QueueConfig=None (not inner Batch=None) so the confmap round-trip
	// doesn't re-enable the exporter batcher via configoptional promotion.
	tr := NewTranslatorWithName("logs", EndpointConfig{LogsEndpoint: "https://logs.us-west-2.amazonaws.com/v1/logs"})

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	otlpCfg := cfg.(*otlphttpexporter.Config)
	assert.False(t, otlpCfg.QueueConfig.HasValue())
}
