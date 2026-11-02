// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logreplayconnector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, component.MustNewType("log_replay"), f.Type())
	assert.Equal(t, component.StabilityLevelAlpha, f.LogsToLogsStability())
	assert.Equal(t, component.StabilityLevelUndefined, f.LogsToMetricsStability())

	set := connectortest.NewNopSettings(f.Type())
	conn, err := f.CreateLogsToLogs(t.Context(), set, testConfig(), consumertest.NewNop())
	require.NoError(t, err)
	require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, conn.Shutdown(t.Context()))

	_, err = f.CreateLogsToMetrics(t.Context(), set, testConfig(), consumertest.NewNop())
	assert.ErrorIs(t, err, pipeline.ErrSignalNotSupported)
}

func TestCreateLogsToLogsErrors(t *testing.T) {
	set := connectortest.NewNopSettings(component.MustNewType(TypeStr))
	_, err := createLogsToLogs(t.Context(), set, struct{}{}, consumertest.NewNop())
	assert.ErrorContains(t, err, "invalid configuration type")

	// A condition that does not compile fails at creation too.
	cfg := testConfig()
	cfg.Rules[0].Conditions = []string{"not ottl"}
	_, err = createLogsToLogs(t.Context(), set, cfg, consumertest.NewNop())
	assert.ErrorContains(t, err, "rules[0] (app_start): conditions:")
}
