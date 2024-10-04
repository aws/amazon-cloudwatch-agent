// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package configprovider

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/service/defaultcomponents"
)

const (
	envRoleARN = "test-role-arn"
	envRegion  = "test-region"
)

func TestConfigProvider(t *testing.T) {
	t.Setenv("ENV_CREDENTIALS_ROLE_ARN", envRoleARN)
	t.Setenv("ENV_REGION", envRegion)
	factories, err := defaultcomponents.Factories()
	require.NoError(t, err)
	providerSettings := GetSettings([]string{filepath.Join("../../translator/tocwconfig/sampleConfig", "config_with_env.yaml")}, zap.NewNop())
	provider, err := otelcol.NewConfigProvider(providerSettings)
	assert.NoError(t, err)
	actualCfg, err := provider.Get(context.Background(), factories)
	assert.NoError(t, err)
	id := component.MustNewIDWithName("awscloudwatchlogs", "emf_logs")
	got, ok := actualCfg.Exporters[id]
	require.True(t, ok)
	gotCfg, ok := got.(*awscloudwatchlogsexporter.Config)
	require.True(t, ok)
	// ENV_LOG_STREAM_NAME isn't set, so it'll resolve to an empty string
	assert.Equal(t, "", gotCfg.LogStreamName)
	assert.Equal(t, envRegion, gotCfg.Region)
	assert.Equal(t, envRoleARN, gotCfg.RoleARN)
}
