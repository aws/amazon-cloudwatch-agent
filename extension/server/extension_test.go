// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
)

type mockEntityStore struct {
	podToServiceEnvironmentMap map[string]entitystore.ServiceEnvironment
}

func newMockEntityStore() *mockEntityStore {
	return &mockEntityStore{
		podToServiceEnvironmentMap: make(map[string]entitystore.ServiceEnvironment),
	}
}

func (es *mockEntityStore) AddPodServiceEnvironmentMapping(podName string, service string, env string) {
	es.podToServiceEnvironmentMap[podName] = entitystore.ServiceEnvironment{
		ServiceName: service,
		Environment: env,
	}
}

func newMockGetPodServiceEnvironmentMapping(es *mockEntityStore) func() map[string]entitystore.ServiceEnvironment {
	return func() map[string]entitystore.ServiceEnvironment {
		return es.podToServiceEnvironmentMap
	}
}

type mockServerConfig struct {
	TLSCert           string
	TLSKey            string
	TLSAllowedCACerts []string
}

func newMockTLSConfig(c *mockServerConfig) func() (*tls.Config, error) {
	return func() (*tls.Config, error) {
		if c.TLSCert == "" && c.TLSKey == "" && len(c.TLSAllowedCACerts) == 0 {
			return nil, nil
		}
		// Mock implementation for testing purposes
		return &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			MinVersion: tls.VersionTLS12,
		}, nil
	}
}

func TestNewServer(t *testing.T) {
	logger, _ := zap.NewProduction()
	config := &Config{
		ListenAddress: ":8080",
	}
	tests := []struct {
		name       string
		want       *Server
		mockSvrCfg *mockServerConfig
		isTLS      bool
	}{
		{
			name: "HTTPSServer",
			want: &Server{
				config: config,
				logger: logger,
			},
			mockSvrCfg: &mockServerConfig{
				TLSCert:           "cert",
				TLSKey:            "key",
				TLSAllowedCACerts: []string{"ca"},
			},
			isTLS: true,
		},
		{
			name: "EmptyHTTPSServer",
			want: &Server{
				config: config,
				logger: logger,
			},
			mockSvrCfg: &mockServerConfig{},
			isTLS:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getTlsConfig = newMockTLSConfig(tt.mockSvrCfg)

			server := NewServer(logger, config)
			assert.NotNil(t, server)
			assert.Equal(t, config, server.config)
			assert.NotNil(t, server.logger)
			if tt.isTLS {
				assert.NotNil(t, server.httpsServer)
				assert.Equal(t, ":8080", server.httpsServer.Addr)
				assert.NotNil(t, server.httpsServer.TLSConfig)
				assert.Equal(t, uint16(tls.VersionTLS12), server.httpsServer.TLSConfig.MinVersion)
				assert.Equal(t, tls.RequireAndVerifyClientCert, server.httpsServer.TLSConfig.ClientAuth)
				assert.NotNil(t, server.httpsServer.Handler)
				assert.Equal(t, 90*time.Second, server.httpsServer.ReadHeaderTimeout)
			} else {
				assert.Nil(t, server.httpsServer)
			}
		})
	}

}

func TestK8sPodToServiceMapHandler(t *testing.T) {
	logger, _ := zap.NewProduction()
	config := &Config{
		ListenAddress: ":8080",
	}
	tests := []struct {
		name     string
		want     map[string]entitystore.ServiceEnvironment
		emptyMap bool
	}{
		{
			name: "HappyPath",
			want: map[string]entitystore.ServiceEnvironment{
				"pod1": {
					ServiceName: "service1",
					Environment: "env1",
				},
				"pod2": {
					ServiceName: "service2",
					Environment: "env2",
				},
			},
		},
		{
			name:     "Empty Map",
			want:     map[string]entitystore.ServiceEnvironment{},
			emptyMap: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(logger, config)
			es := newMockEntityStore()
			getPodServiceEnvironmentMapping = newMockGetPodServiceEnvironmentMapping(es)
			if !tt.emptyMap {
				es.AddPodServiceEnvironmentMapping("pod1", "service1", "env1")
				es.AddPodServiceEnvironmentMapping("pod2", "service2", "env2")
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			server.k8sPodToServiceMapHandler(c)

			assert.Equal(t, http.StatusOK, w.Code)

			var actualMap map[string]entitystore.ServiceEnvironment
			err := json.Unmarshal(w.Body.Bytes(), &actualMap)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, actualMap)
		})
	}
}

func TestJSONHandler(t *testing.T) {

	tests := []struct {
		name         string
		expectedData map[string]string
	}{
		{
			name:         "EmptyData",
			expectedData: map[string]string{},
		},
		{
			name: "NonEmptyData",
			expectedData: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewProduction()
			config := &Config{
				ListenAddress: ":8080",
			}
			server := NewServer(logger, config)
			w := httptest.NewRecorder()
			server.jsonHandler(w, tt.expectedData)
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var actualData map[string]string
			err := json.Unmarshal(w.Body.Bytes(), &actualData)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedData, actualData)
		})
	}
}

func TestServerStartAndShutdown(t *testing.T) {
	logger, _ := zap.NewProduction()
	config := &Config{
		ListenAddress: ":8080",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tests := []struct {
		name       string
		mockSvrCfg *mockServerConfig
	}{
		{
			name: "HTTPSServer",
			mockSvrCfg: &mockServerConfig{
				TLSCert:           "cert",
				TLSKey:            "key",
				TLSAllowedCACerts: []string{"ca"},
			},
		},
		{
			name:       "EmptyHTTPSServer",
			mockSvrCfg: &mockServerConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getTlsConfig = newMockTLSConfig(tt.mockSvrCfg)
			server := NewServer(logger, config)

			err := server.Start(ctx, nil)
			assert.NoError(t, err)

			time.Sleep(1 * time.Second)

			err = server.Shutdown(ctx)
			assert.NoError(t, err)
		})
	}
}
