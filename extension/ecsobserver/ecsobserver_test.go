// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package ecsobserver

import (
	"context"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

func TestECSObserver(t *testing.T) {
	// Create a test configuration
	config := &ecsobserver.Config{
		RefreshInterval: 10 * time.Second,
		ClusterName:     "test-cluster",
		ClusterRegion:   "us-west-2",
		ResultFile:      "/tmp/ecs_observer_result.yaml",
		// Add at least one matcher to satisfy the validation
		Services: []ecsobserver.ServiceConfig{
			{
				NamePattern: "test-service",
				CommonExporterConfig: ecsobserver.CommonExporterConfig{
					JobName:      "test-job",
					MetricsPath:  "/metrics",
					MetricsPorts: []int{9090},
				},
			},
		},
	}

	// Create a test logger
	logger := zap.NewNop()

	// Create telemetry settings
	settings := componenttest.NewNopTelemetrySettings()

	// Create the ECS observer extension
	observer, err := NewECSObserver(config, logger, settings)
	require.NoError(t, err)
	require.NotNil(t, observer)

	// Verify the configuration is correctly set
	assert.Equal(t, config, observer.GetConfig())

	// Test the Start and Shutdown methods (these will be no-ops in the test)
	host := componenttest.NewNopHost()
	err = observer.Start(context.Background(), host)
	assert.NoError(t, err)

	err = observer.Shutdown(context.Background())
	assert.NoError(t, err)
}
