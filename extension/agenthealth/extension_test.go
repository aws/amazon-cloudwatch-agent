// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

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
