// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscontainerinsightskueue

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightskueuereceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

func TestTranslator(t *testing.T) {
	acit := NewTranslator()
	require.EqualValues(t, "awscontainerinsightskueuereceiver", acit.ID().String())
	testCases := map[string]struct {
		input     map[string]interface{}
		isSystemd bool
		want      *awscontainerinsightskueuereceiver.Config
		wantErr   error
	}{
		"WithClusterName": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"cluster_name":             "TestCluster",
							"kueue_container_insights": true,
						},
					},
				},
			},
			isSystemd: true,
			want: &awscontainerinsightskueuereceiver.Config{
				CollectionInterval: defaultMetricsCollectionInterval,
				ClusterName:        "TestCluster",
			},
		},
		"WithClusterNameAndCollectionInterval": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"cluster_name":             "TestCluster",
							"kueue_container_insights": true,
						},
					},
				},
				"agent": map[string]interface{}{
					"metrics_collection_interval": 30,
				},
			},
			isSystemd: true,
			want: &awscontainerinsightskueuereceiver.Config{
				CollectionInterval: 30 * time.Second,
				ClusterName:        "TestCluster",
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			context.CurrentContext().SetRunInContainer(!testCase.isSystemd)
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := acit.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*awscontainerinsightskueuereceiver.Config)
				require.True(t, ok)
				require.Equal(t, testCase.want.CollectionInterval, gotCfg.CollectionInterval)
				require.Equal(t, testCase.want.ClusterName, gotCfg.ClusterName)
			}
		})
	}
}
