// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awscontainerinsight

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	acit := NewTranslator()
	require.EqualValues(t, "awscontainerinsightreceiver", acit.Type())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *awscontainerinsightreceiver.Config
		wantErr error
	}{
		"WithoutECSOrKubernetesKeys": {
			input: map[string]interface{}{},
			wantErr: &common.MissingKeyError{
				Type:    acit.Type(),
				JsonKey: "logs::metrics_collected::ecs or logs::metrics_collected::kubernetes",
			},
		},
		"WithECS/WithoutInterval": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"ecs": map[string]interface{}{},
					},
				},
			},
			want: &awscontainerinsightreceiver.Config{
				ContainerOrchestrator: ecs,
				CollectionInterval:    time.Minute,
			},
		},
		"WithECS/WithAgentInterval": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"metrics_collection_interval": float64(20),
				},
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"ecs": map[string]interface{}{},
					},
				},
			},
			want: &awscontainerinsightreceiver.Config{
				ContainerOrchestrator: ecs,
				CollectionInterval:    20 * time.Second,
			},
		},
		"WithECS/WithSectionInterval": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"metrics_collection_interval": float64(20),
				},
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"ecs": map[string]interface{}{
							"metrics_collection_interval": float64(10),
						},
					},
				},
			},
			want: &awscontainerinsightreceiver.Config{
				ContainerOrchestrator: ecs,
				CollectionInterval:    10 * time.Second,
			},
		},
		"WithKubernetes": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"metrics_collection_interval": float64(10),
						},
					},
				},
			},
			want: &awscontainerinsightreceiver.Config{
				ContainerOrchestrator: eks,
				CollectionInterval:    10 * time.Second,
			},
		},
		"WithECSAndKubernetes": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"ecs": map[string]interface{}{
							"metrics_collection_interval": float64(5),
						},
						"kubernetes": map[string]interface{}{
							"metrics_collection_interval": float64(10),
						},
					},
				},
			},
			want: &awscontainerinsightreceiver.Config{
				ContainerOrchestrator: ecs,
				CollectionInterval:    5 * time.Second,
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := acit.Translate(conf, common.TranslatorOptions{})
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*awscontainerinsightreceiver.Config)
				require.True(t, ok)
				require.Equal(t, testCase.want.ContainerOrchestrator, gotCfg.ContainerOrchestrator)
				require.Equal(t, testCase.want.CollectionInterval, gotCfg.CollectionInterval)
			}
		})
	}
}
