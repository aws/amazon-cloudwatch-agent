// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package stats

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

type mockStatsProvider struct {
	stats *agent.Stats
}

var _ agent.StatsProvider = (*mockStatsProvider)(nil)

func (m *mockStatsProvider) Stats(string) agent.Stats {
	return *m.stats
}

func newMockStatsProvider(stats *agent.Stats) agent.StatsProvider {
	return &mockStatsProvider{stats: stats}
}

func TestStatsHandler(t *testing.T) {
	stats := &agent.Stats{
		FileDescriptorCount:  aws.Int32(456),
		ThreadCount:          aws.Int32(789),
		LatencyMillis:        aws.Int64(1234),
		PayloadBytes:         aws.Int(5678),
		StatusCode:           aws.Int(200),
		ImdsFallbackSucceed:  aws.Int(1),
		SharedConfigFallback: aws.Int(1),
	}
	handler := newStatsHandler(
		zap.NewNop(),
		[]agent.StatsProvider{
			newMockStatsProvider(&agent.Stats{CpuPercent: aws.Float64(1.2)}),
			newMockStatsProvider(&agent.Stats{MemoryBytes: aws.Uint64(123)}),
			newMockStatsProvider(stats),
		},
	)
	ctx := context.Background()
	assert.Equal(t, awsmiddleware.After, handler.Position())
	assert.Equal(t, handlerID, handler.ID())
	req, err := http.NewRequest("", "localhost", nil)
	require.NoError(t, err)
	handler.HandleRequest(ctx, req)
	assert.Equal(t, "", req.Header.Get(headerKeyAgentStats))
	handler.HandleResponse(ctx, &http.Response{StatusCode: http.StatusOK})
	time.Sleep(time.Millisecond)
	handler.HandleRequest(ctx, req)
	assert.Equal(t, `"cpu":1.2,"mem":123,"fd":456,"th":789,"lat":1234,"load":5678,"code":200,"scfb":1,"ifs":1`, req.Header.Get(headerKeyAgentStats))
	stats.StatusCode = aws.Int(404)
	stats.LatencyMillis = nil
	handler.HandleResponse(ctx, &http.Response{StatusCode: http.StatusOK})
	time.Sleep(time.Millisecond)
	handler.HandleRequest(ctx, req)
	assert.Equal(t, `"cpu":1.2,"mem":123,"fd":456,"th":789,"load":5678,"code":404,"scfb":1,"ifs":1`, req.Header.Get(headerKeyAgentStats))
}
