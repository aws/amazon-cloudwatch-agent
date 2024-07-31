// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/aws-sdk-go/aws"
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
	req.ContentLength = 20
	ctx := context.Background()
	handler.HandleRequest(ctx, req)
	got := handler.Stats(operation)
	assert.Nil(t, got.LatencyMillis)
	assert.Nil(t, got.PayloadBytes)
	assert.Nil(t, got.StatusCode)
	time.Sleep(time.Millisecond)
	handler.HandleResponse(ctx, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(`{"rejectedEntityInfo":{"errorType":"InvalidAttributes"}}`)),
	})
	got = handler.Stats(operation)
	assert.NotNil(t, got.LatencyMillis)
	assert.NotNil(t, got.PayloadBytes)
	assert.NotNil(t, got.StatusCode)
	assert.NotNil(t, got.EntityRejected)
	assert.Equal(t, 1, *got.EntityRejected)
	assert.Equal(t, http.StatusOK, *got.StatusCode)
	assert.Equal(t, 20, *got.PayloadBytes)
	assert.GreaterOrEqual(t, *got.LatencyMillis, int64(1))

	// without content length
	req.ContentLength = 0
	handler.HandleRequest(ctx, req)
	handler.HandleResponse(ctx, &http.Response{StatusCode: http.StatusOK})
	got = handler.Stats(operation)
	assert.NotNil(t, got.PayloadBytes)
	assert.Equal(t, 17, *got.PayloadBytes)

	// with seeker
	body = append(body, " with seeker"...)
	req, err = http.NewRequest("", "localhost", aws.ReadSeekCloser(bytes.NewReader(body)))
	require.NoError(t, err)
	req.ContentLength = 0
	handler.HandleRequest(ctx, req)
	handler.HandleResponse(ctx, &http.Response{StatusCode: http.StatusOK})
	got = handler.Stats(operation)
	assert.NotNil(t, got.PayloadBytes)
	assert.Equal(t, 29, *got.PayloadBytes)
}

func BenchmarkRejectedEntityInfoExists(b *testing.B) {
	body := `{"rejectedEntityInfo":{"errorType":"InvalidAttributes"}}`
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewBufferString(body)),
	}

	for n := 0; n < b.N; n++ {
		rejectedEntityInfoExists(resp)
		// Reset the body for the next iteration
		resp.Body = io.NopCloser(bytes.NewBufferString(body))
	}
}
