// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
)

type mockEntityStore struct {
	podToServiceEnvironmentMap *ttlcache.Cache[string, entitystore.ServiceEnvironment]
}

// This helper function creates a test logger
// so that it can send the log messages into a
// temporary buffer for pattern matching
func CreateTestLogger(buf *bytes.Buffer) *zap.Logger {
	writer := zapcore.AddSync(buf)

	// Create a custom zapcore.Core that writes to the buffer
	encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(encoder, writer, zapcore.DebugLevel)
	logger := zap.New(core)
	return logger
}

func newMockEntityStore() *mockEntityStore {
	return &mockEntityStore{
		podToServiceEnvironmentMap: ttlcache.New[string, entitystore.ServiceEnvironment](
			ttlcache.WithTTL[string, entitystore.ServiceEnvironment](time.Hour),
		),
	}
}

func (es *mockEntityStore) AddPodServiceEnvironmentMapping(podName string, service string, env string, serviceSource string) {
	es.podToServiceEnvironmentMap.Set(podName, entitystore.ServiceEnvironment{
		ServiceName:       service,
		Environment:       env,
		ServiceNameSource: serviceSource,
	}, time.Hour)
}

func (es *mockEntityStore) GetPodServiceEnvironmentMapping() *ttlcache.Cache[string, entitystore.ServiceEnvironment] {
	return es.podToServiceEnvironmentMap
}

func newMockGetPodServiceEnvironmentMapping(es *mockEntityStore) func() *ttlcache.Cache[string, entitystore.ServiceEnvironment] {
	return func() *ttlcache.Cache[string, entitystore.ServiceEnvironment] {
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
	tests := []struct {
		name   string
		want   *Server
		config *Config
		isTLS  bool
	}{
		{
			name: "Should load valid HTTPS server",
			want: &Server{
				logger: logger,
			},
			config: &Config{
				TLSCertPath:   "./testdata/example-server-cert.pem",
				TLSKeyPath:    "./testdata/example-server-key.pem",
				TLSCAPath:     "./testdata/example-CA-cert.pem",
				ListenAddress: ":8080",
			},
			isTLS: true,
		},
		{
			name: "should load server with empty HTTPS server as certs are empty",
			want: &Server{
				logger: logger,
			},
			config: &Config{
				ListenAddress: ":8080",
			},
			isTLS: false,
		},
		{
			name: "should load server with empty HTTPS server as CA cert is not valid",
			want: &Server{
				logger: logger,
			},
			config: &Config{
				TLSCertPath:   "./testdata/example-server-cert.pem",
				TLSKeyPath:    "./testdata/example-server-key.pem",
				TLSCAPath:     "./testdata/bad-CA-cert.pem",
				ListenAddress: ":8080",
			},
			isTLS: false,
		},
		{
			name: "should load server with empty HTTPS server as server cert is not valid",
			want: &Server{
				logger: logger,
			},
			config: &Config{
				TLSCertPath:   "./testdata/bad-CA-cert.pem",
				TLSKeyPath:    "./testdata/example-server-key.pem",
				TLSCAPath:     "./testdata/example-CA-cert.pem",
				ListenAddress: ":8080",
			},
			isTLS: false,
		},
		{
			name: "should load server with empty HTTPS server as server key is empty",
			want: &Server{
				logger: logger,
			},
			config: &Config{
				TLSCertPath:   "./testdata/example-server-cert.pem",
				TLSKeyPath:    "",
				TLSCAPath:     "./testdata/example-CA-cert.pem",
				ListenAddress: ":8080",
			},
			isTLS: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(logger, tt.config)
			assert.NotNil(t, server)
			assert.Equal(t, tt.config, server.config)
			assert.NotNil(t, server.logger)
			if tt.isTLS {
				assert.NotNil(t, server.httpsServer)
				assert.Equal(t, ":8080", server.httpsServer.Addr)
				assert.NotNil(t, server.httpsServer.TLSConfig)
				assert.NotNil(t, server.httpsServer.TLSConfig.GetCertificate)
				assert.NotNil(t, server.httpsServer.TLSConfig.ClientCAs)
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
		want     *ttlcache.Cache[string, entitystore.ServiceEnvironment]
		emptyMap bool
	}{
		{
			name: "HappyPath",
			want: setupTTLCacheForTesting(map[string]entitystore.ServiceEnvironment{
				"pod1": {
					ServiceName:       "service1",
					Environment:       "env1",
					ServiceNameSource: "source1",
				},
				"pod2": {
					ServiceName:       "service2",
					Environment:       "env2",
					ServiceNameSource: "source2",
				},
			}),
		},
		{
			name:     "Empty Map",
			want:     setupTTLCacheForTesting(map[string]entitystore.ServiceEnvironment{}),
			emptyMap: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(logger, config)
			es := newMockEntityStore()
			getPodServiceEnvironmentMapping = newMockGetPodServiceEnvironmentMapping(es)
			if !tt.emptyMap {
				es.AddPodServiceEnvironmentMapping("pod1", "service1", "env1", "source1")
				es.AddPodServiceEnvironmentMapping("pod2", "service2", "env2", "source2")
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			server.k8sPodToServiceMapHandler(c)

			assert.Equal(t, http.StatusOK, w.Code)

			var actualMap map[string]entitystore.ServiceEnvironment
			err := json.Unmarshal(w.Body.Bytes(), &actualMap)
			assert.NoError(t, err)
			actualTtlCache := setupTTLCacheForTesting(actualMap)
			for pod, se := range tt.want.Items() {
				assert.Equal(t, se.Value(), actualTtlCache.Get(pod).Value())
			}
			assert.Equal(t, tt.want.Len(), actualTtlCache.Len())
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tests := []struct {
		name   string
		config *Config
	}{
		{
			name: "HTTPSServer",
			config: &Config{
				TLSCertPath:   "./testdata/example-server-cert.pem",
				TLSKeyPath:    "./testdata/example-server-key.pem",
				TLSCAPath:     "./testdata/example-CA-cert.pem",
				ListenAddress: ":8080",
			},
		},
		{
			name: "EmptyHTTPSServer",
			config: &Config{
				TLSCertPath:   "./testdata/example-server-cert.pem",
				TLSKeyPath:    "./testdata/example-server-key.pem",
				TLSCAPath:     "./testdata/bad-CA-cert.pem",
				ListenAddress: ":8080",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(logger, tt.config)

			err := server.Start(ctx, nil)
			assert.NoError(t, err)

			time.Sleep(1 * time.Second)

			err = server.Shutdown(ctx)
			assert.NoError(t, err)
		})
	}
}

func TestConvertTtlCacheToMap(t *testing.T) {
	podToServiceMap := map[string]entitystore.ServiceEnvironment{
		"pod1": {
			ServiceName: "service1",
			Environment: "env1",
		},
		"pod2": {
			ServiceName: "service2",
			Environment: "env2",
		},
	}
	ttlcache := setupTTLCacheForTesting(podToServiceMap)
	convertedMap := convertTtlCacheToMap(ttlcache)
	assert.Equal(t, convertedMap, podToServiceMap)
}

func setupTTLCacheForTesting(podToServiceMap map[string]entitystore.ServiceEnvironment) *ttlcache.Cache[string, entitystore.ServiceEnvironment] {
	cache := ttlcache.New[string, entitystore.ServiceEnvironment](ttlcache.WithTTL[string, entitystore.ServiceEnvironment](time.Minute))
	for pod, serviceEnv := range podToServiceMap {
		cache.Set(pod, serviceEnv, ttlcache.DefaultTTL)
	}
	return cache
}

func TestServerNoSensitiveInfoInLogs(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	logger := CreateTestLogger(&buf)

	config := &Config{
		TLSCertPath:   "./testdata/example-server-cert.pem",
		TLSKeyPath:    "./testdata/example-server-key.pem",
		TLSCAPath:     "./testdata/example-CA-cert.pem",
		ListenAddress: ":8080",
	}

	tests := []struct {
		name          string
		setupMockData func(*mockEntityStore)
	}{
		{
			name:          "EmptyPodServiceMap",
			setupMockData: func(es *mockEntityStore) {},
		},
		{
			name: "PopulatedPodServiceMap",
			setupMockData: func(es *mockEntityStore) {
				es.AddPodServiceEnvironmentMapping("pod1", "service1", "env1", "source1")
				es.AddPodServiceEnvironmentMapping("pod2", "service2", "env2", "source2")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear the buffer before each test
			buf.Reset()

			server := NewServer(logger, config)
			es := newMockEntityStore()
			tt.setupMockData(es)
			getPodServiceEnvironmentMapping = newMockGetPodServiceEnvironmentMapping(es)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			server.k8sPodToServiceMapHandler(c)

			// Check logs for sensitive information
			logOutput := buf.String()
			assertNoSensitiveInfo(t, logOutput, config, es)
		})
	}
}

func assertNoSensitiveInfo(t *testing.T, logOutput string, config *Config, es *mockEntityStore) {
	confidentialInfo := []string{
		"-----BEGIN CERTIFICATE-----",
		"-----END CERTIFICATE-----",
		"-----BEGIN RSA PRIVATE KEY-----",
		"-----END RSA PRIVATE KEY-----",
	}

	for _, pattern := range confidentialInfo {
		assert.NotContains(t, logOutput, pattern)
	}

	// Iterate through the pod service environment mapping
	podServiceMap := es.GetPodServiceEnvironmentMapping()
	for pod, serviceEnv := range podServiceMap.Items() {
		assert.NotContains(t, logOutput, pod)
		assert.NotContains(t, logOutput, serviceEnv.Value().ServiceName)
		assert.NotContains(t, logOutput, serviceEnv.Value().Environment)
		assert.NotContains(t, logOutput, serviceEnv.Value().ServiceNameSource)
	}
}
