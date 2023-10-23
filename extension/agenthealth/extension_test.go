// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

func TestExtension(t *testing.T) {
	ctx := context.Background()
	extension, err := newAgentHealth(zap.NewNop(), &Config{IsUsageDataEnabled: true})
	assert.NoError(t, err)
	assert.NotNil(t, extension)
	assert.NoError(t, extension.Start(ctx, componenttest.NewNopHost()))
	requests, responses := extension.Handlers()
	assert.Len(t, requests, 1)
	assert.Len(t, responses, 0)
	extension.cfg.IsUsageDataEnabled = false
	extension.Handlers()
	requests, responses = extension.Handlers()
	assert.Len(t, requests, 1)
	assert.Len(t, responses, 0)
	assert.NoError(t, extension.Shutdown(ctx))
}
