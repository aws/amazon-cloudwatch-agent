// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package server

import (
	"context"
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
func TestNewServer(t *testing.T) {
	logger, _ := zap.NewProduction()
	config := &Config{
		ListenAddress: ":8080",
	}
	server := NewServer(logger, config)

	assert.NotNil(t, server)
	assert.Equal(t, config, server.config)
	assert.NotNil(t, server.logger)
	assert.NotNil(t, server.server)
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
	server := NewServer(logger, config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Start(ctx, nil)
	assert.NoError(t, err)

	time.Sleep(1 * time.Second)

	// Make a request to the server to check if it's running
	resp, err := http.Get("http://localhost:8080")
	assert.NoError(t, err)
	defer resp.Body.Close()

	// Check if the response status code is 404 (default route)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	err = server.Shutdown(ctx)
	assert.NoError(t, err)

	// Wait for the server to shut down
	time.Sleep(1 * time.Second)

	// Make a request to the server to check if it's shutdown
	_, err = http.Get("http://localhost:8080")
	assert.Error(t, err)
}
