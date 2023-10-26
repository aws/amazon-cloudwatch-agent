// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package client

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

func TestHandle(t *testing.T) {
	operation := "test"
	handler := NewHandler(agent.NewOperationsFilter("test"))
	handler.(*clientStatsHandler).getOperationName = func(context.Context) string {
		return operation
	}
	assert.Equal(t, awsmiddleware.After, handler.Position())
	assert.Equal(t, handlerID, handler.ID())
	body := []byte("test payload size")
	req, err := http.NewRequest("", "localhost", bytes.NewBuffer(body))
	require.NoError(t, err)
	ctx := context.Background()
	handler.HandleRequest(ctx, req)
	got := handler.Stats(operation)
	assert.Nil(t, got.LatencyMillis)
	assert.Nil(t, got.PayloadBytes)
	assert.Nil(t, got.StatusCode)
	time.Sleep(time.Millisecond)
	handler.HandleResponse(ctx, &http.Response{StatusCode: http.StatusOK})
	got = handler.Stats(operation)
	assert.NotNil(t, got.LatencyMillis)
	assert.NotNil(t, got.PayloadBytes)
	assert.NotNil(t, got.StatusCode)
	assert.Equal(t, http.StatusOK, *got.StatusCode)
	assert.Equal(t, 17, *got.PayloadBytes)
	assert.GreaterOrEqual(t, *got.LatencyMillis, int64(1))
}
