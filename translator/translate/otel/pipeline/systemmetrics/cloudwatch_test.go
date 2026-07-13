// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package systemmetrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/cloudwatch"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/extension/agenthealth"
)

func TestCloudWatchTranslator(t *testing.T) {
	tt := newCloudWatchTranslator(true)
	assert.Equal(t, "awscloudwatch/"+common.PipelineNameSystemMetrics, tt.ID().String())

	got, err := tt.Translate(nil)
	require.NoError(t, err)
	cfg, ok := got.(*cloudwatch.Config)
	require.True(t, ok)
	assert.Equal(t, "CWAgent/System", cfg.Namespace)
	assert.Equal(t, [][]string{{"InstanceId"}, {}}, cfg.RollupDimensions)
	assert.Equal(t, &agenthealth.MetricsID, cfg.MiddlewareID)
	assert.Equal(t, 2, cfg.MaxRetryCount)
	assert.Equal(t, time.Minute, cfg.BackoffRetryBase)
	assert.Equal(t, 1, cfg.MaxConcurrentPublishers)
}

func TestCloudWatchTranslatorNotEC2(t *testing.T) {
	tt := newCloudWatchTranslator(false)

	got, err := tt.Translate(nil)
	require.NoError(t, err)
	cfg, ok := got.(*cloudwatch.Config)
	require.True(t, ok)
	assert.Equal(t, [][]string{{}}, cfg.RollupDimensions)
}
