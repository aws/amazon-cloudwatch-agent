// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package transformprocessor

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestNewSetClusterNameTranslator(t *testing.T) {
	t.Setenv("K8S_CLUSTER_NAME", "")
	testCases := map[string]struct {
		clusterName string
		wantErr     bool
	}{
		"Valid":   {clusterName: "test-cluster"},
		"Invalid": {clusterName: "bad cluster name!", wantErr: true},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(map[string]interface{}{
				"opentelemetry": map[string]interface{}{"cluster_name": testCase.clusterName},
			})
			tr, err := NewSetClusterNameTranslator(conf)
			if testCase.wantErr {
				require.Error(t, err)
				assert.Nil(t, tr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, tr)
			assert.Equal(t, "transform/set_cluster_name", tr.ID().String())

			cfg, err := tr.Translate(conf)
			require.NoError(t, err)
			actualCfg := cfg.(*transformprocessor.Config)
			want := []string{`set(resource.attributes["k8s.cluster.name"], "test-cluster")`}
			require.Len(t, actualCfg.MetricStatements, 1)
			require.Len(t, actualCfg.LogStatements, 1)
			require.Len(t, actualCfg.TraceStatements, 1)
			require.Len(t, actualCfg.ProfileStatements, 1)
			for _, block := range []struct {
				context    string
				statements []string
			}{
				{string(actualCfg.MetricStatements[0].Context), actualCfg.MetricStatements[0].Statements},
				{string(actualCfg.LogStatements[0].Context), actualCfg.LogStatements[0].Statements},
				{string(actualCfg.TraceStatements[0].Context), actualCfg.TraceStatements[0].Statements},
				{string(actualCfg.ProfileStatements[0].Context), actualCfg.ProfileStatements[0].Statements},
			} {
				assert.Equal(t, "resource", block.context)
				assert.Equal(t, want, block.statements)
			}
			require.NoError(t, actualCfg.Validate())
		})
	}
}
