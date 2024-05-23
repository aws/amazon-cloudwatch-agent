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

	actualProvider, err := Get(filepath.Join("../../translator/tocwconfig/sampleConfig", "config_with_env.yaml"))
	assert.NoError(t, err)
	actualCfg, err := actualProvider.Get(context.Background(), factories)
	assert.NoError(t, err)
	cloudwatchType, _ := component.NewType("awscloudwatchlogs")
	got, ok := actualCfg.Exporters[component.NewIDWithName(cloudwatchType, "emf_logs")]
	require.True(t, ok)
	gotCfg, ok := got.(*awscloudwatchlogsexporter.Config)
	require.True(t, ok)
	// ENV_LOG_STREAM_NAME isn't set, so it'll resolve to an empty string
	assert.Equal(t, "", gotCfg.LogStreamName)
	assert.Equal(t, envRegion, gotCfg.Region)
	assert.Equal(t, envRoleARN, gotCfg.RoleARN)
}
