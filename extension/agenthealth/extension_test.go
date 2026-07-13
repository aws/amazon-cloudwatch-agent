// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

type mockAuthExtension struct {
	component.StartFunc
	component.ShutdownFunc
	called bool
	err    error
}

func (m *mockAuthExtension) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	return base, nil
}

var _ extensionauth.HTTPClient = (*mockAuthExtension)(nil)

func TestExtension(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{IsUsageDataEnabled: true, IsStatusCodeEnabled: true, Stats: &agent.StatsConfig{Operations: []string{"ListBuckets"}}}
	extension := NewAgentHealth(zap.NewNop(), cfg)
	assert.NotNil(t, extension)
	assert.NoError(t, extension.Start(ctx, componenttest.NewNopHost()))
	requestHandlers, responseHandlers := extension.Handlers()
	// user agent, client stats, stats
	assert.Len(t, requestHandlers, 3)
	// client stats
	assert.Len(t, responseHandlers, 2)
	cfg.IsUsageDataEnabled = false
	requestHandlers, responseHandlers = extension.Handlers()
	// user agent
	assert.Len(t, requestHandlers, 1)
	assert.Len(t, responseHandlers, 0)
	assert.NoError(t, extension.Shutdown(ctx))
}

func TestExtensionStatusCodeOnly(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{IsUsageDataEnabled: true, IsStatusCodeEnabled: true}
	extension := NewAgentHealth(zap.NewNop(), cfg)
	assert.NotNil(t, extension)
	assert.NoError(t, extension.Start(ctx, componenttest.NewNopHost()))
	requestHandlers, responseHandlers := extension.Handlers()
	// user agent, client stats, stats
	assert.Len(t, requestHandlers, 1)
	// client stats
	assert.Len(t, responseHandlers, 1)
	cfg.IsUsageDataEnabled = false
	requestHandlers, responseHandlers = extension.Handlers()
	// user agent
	assert.Len(t, requestHandlers, 1)
	assert.Len(t, responseHandlers, 0)
	assert.NoError(t, extension.Shutdown(ctx))
}

func TestDependencies_Nil(t *testing.T) {
	cfg := &Config{IsUsageDataEnabled: true}
	ext := NewAgentHealth(zap.NewNop(), cfg)
	assert.Nil(t, ext.(extensioncapabilities.Dependent).Dependencies())
}

func TestDependencies_WithAdditionalAuth(t *testing.T) {
	authID := component.NewID(component.MustNewType("sigv4auth"))
	cfg := &Config{IsUsageDataEnabled: true, AdditionalAuth: &authID}
	ext := NewAgentHealth(zap.NewNop(), cfg)
	deps := ext.(extensioncapabilities.Dependent).Dependencies()
	assert.Equal(t, []component.ID{authID}, deps)
}

func TestRoundTripper_NoInnerAuth(t *testing.T) {
	cfg := &Config{IsUsageDataEnabled: true}
	ext := NewAgentHealth(zap.NewNop(), cfg)
	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

	var baseCalled bool
	base := RoundTripFunc(func(*http.Request) (*http.Response, error) {
		baseCalled = true
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	})

	rt, err := ext.(extensionauth.HTTPClient).RoundTripper(base)
	require.NoError(t, err)
	assert.NotNil(t, rt)

	req, err := http.NewRequest(http.MethodPost, "http://localhost/v1/metrics", nil)
	require.NoError(t, err)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.True(t, baseCalled)
}

func TestRoundTripper_WithInnerAuth(t *testing.T) {
	mockAuth := &mockAuthExtension{}
	authID := component.NewID(component.MustNewType("mockauth"))

	mockHost := &awsmiddleware.MockExtensionsHost{}
	mockHost.On("GetExtensions").Return(map[component.ID]component.Component{
		authID: mockAuth,
	})

	cfg := &Config{
		IsUsageDataEnabled: true,
		AdditionalAuth:     &authID,
	}
	ext := NewAgentHealth(zap.NewNop(), cfg)
	require.NoError(t, ext.Start(context.Background(), mockHost))

	base := RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	})

	rt, err := ext.(extensionauth.HTTPClient).RoundTripper(base)
	require.NoError(t, err)
	assert.NotNil(t, rt)
	assert.True(t, mockAuth.called)
	mockHost.AssertExpectations(t)
}

func TestRoundTripper_InnerAuthError(t *testing.T) {
	mockAuth := &mockAuthExtension{err: errors.New("auth failed")}
	authID := component.NewID(component.MustNewType("mockauth"))

	mockHost := &awsmiddleware.MockExtensionsHost{}
	mockHost.On("GetExtensions").Return(map[component.ID]component.Component{
		authID: mockAuth,
	})

	cfg := &Config{
		IsUsageDataEnabled: true,
		AdditionalAuth:     &authID,
	}
	ext := NewAgentHealth(zap.NewNop(), cfg)
	require.NoError(t, ext.Start(context.Background(), mockHost))

	rt, err := ext.(extensionauth.HTTPClient).RoundTripper(http.DefaultTransport)
	assert.Error(t, err)
	assert.Nil(t, rt)
	assert.True(t, mockAuth.called)
}

func TestRoundTripper_ExtensionNotFound(t *testing.T) {
	authID := component.NewID(component.MustNewType("mockauth"))

	mockHost := &awsmiddleware.MockExtensionsHost{}
	mockHost.On("GetExtensions").Return(map[component.ID]component.Component{})

	cfg := &Config{IsUsageDataEnabled: true, AdditionalAuth: &authID}
	ext := NewAgentHealth(zap.NewNop(), cfg)
	require.NoError(t, ext.Start(context.Background(), mockHost))

	rt, err := ext.(extensionauth.HTTPClient).RoundTripper(http.DefaultTransport)
	assert.ErrorContains(t, err, "not found")
	assert.Nil(t, rt)
}

func TestRoundTripper_NotHTTPClient(t *testing.T) {
	authID := component.NewID(component.MustNewType("mockauth"))

	type nonHTTPExtension struct {
		component.StartFunc
		component.ShutdownFunc
	}
	mockHost := &awsmiddleware.MockExtensionsHost{}
	mockHost.On("GetExtensions").Return(map[component.ID]component.Component{
		authID: &nonHTTPExtension{},
	})

	cfg := &Config{IsUsageDataEnabled: true, AdditionalAuth: &authID}
	ext := NewAgentHealth(zap.NewNop(), cfg)
	require.NoError(t, ext.Start(context.Background(), mockHost))

	rt, err := ext.(extensionauth.HTTPClient).RoundTripper(http.DefaultTransport)
	assert.ErrorContains(t, err, "does not implement extensionauth.HTTPClient")
	assert.Nil(t, rt)
}
