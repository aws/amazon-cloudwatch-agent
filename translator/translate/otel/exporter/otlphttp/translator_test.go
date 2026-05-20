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
	assert.Equal(t, "otlphttp/test", tr.ID().String())
}

func TestTranslatorWithEndpoints(t *testing.T) {
	endpoint := EndpointConfig{
		BaseEndpoint:    "https://logs.us-west-2.amazonaws.com",
		LogsEndpoint:    "https://logs.us-west-2.amazonaws.com/v1/logs",
		MetricsEndpoint: "https://monitoring.us-west-2.amazonaws.com/v1/metrics",
		TracesEndpoint:  "https://xray.us-west-2.amazonaws.com/v1/traces",
	}
	tr := NewTranslatorWithName("full", endpoint)

	cfg, err := tr.Translate(nil)
	require.NoError(t, err)

	otlpCfg := cfg.(*otlphttpexporter.Config)
	assert.Equal(t, "https://logs.us-west-2.amazonaws.com", otlpCfg.ClientConfig.Endpoint)
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
	require.NotNil(t, otlpCfg.ClientConfig.Auth)
	assert.Equal(t, authID, otlpCfg.ClientConfig.Auth.AuthenticatorID)
}
