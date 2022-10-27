// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsemf

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"github.com/stretchr/testify/require"
)

func TestTranslator(t *testing.T) {
	tt := NewTranslator()
	require.EqualValues(t, "awsemf", tt.Type())
	got, err := tt.Translate(nil)
	require.NoError(t, err)
	gotCfg := got.(*awsemfexporter.Config)
	require.Equal(t, "ECS/ContainerInsights", gotCfg.Namespace)
	require.Equal(t, "/aws/ecs/containerinsights/{ClusterName}/performance", gotCfg.LogGroupName)
	require.Equal(t, "instanceTelemetry/{ContainerInstanceId}", gotCfg.LogStreamName)
	require.Len(t, gotCfg.MetricDeclarations, 2)
}
