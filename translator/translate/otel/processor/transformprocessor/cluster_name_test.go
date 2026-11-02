// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package transformprocessor

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/util/tagutil"
)

type noTagsEC2Client struct{}

func (noTagsEC2Client) DescribeTags(context.Context, *ec2.DescribeTagsInput, ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
	return &ec2.DescribeTagsOutput{}, nil
}

func TestNewSetClusterNameTranslator(t *testing.T) {
	t.Setenv("K8S_CLUSTER_NAME", "")
	tagutil.SetEC2APIProviderForTesting(func() ec2.DescribeTagsAPIClient { return noTagsEC2Client{} })
	t.Cleanup(tagutil.ResetEC2APIProvider)
	t.Cleanup(tagutil.ResetTagsCache)
	testCases := map[string]struct {
		clusterName string
		wantErr     bool
		wantNil     bool
	}{
		"Valid":   {clusterName: "test-cluster"},
		"Invalid": {clusterName: "bad cluster name!", wantErr: true},
		"Absent":  {clusterName: "", wantNil: true},
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
			if testCase.wantNil {
				assert.Nil(t, tr)
				return
			}
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
