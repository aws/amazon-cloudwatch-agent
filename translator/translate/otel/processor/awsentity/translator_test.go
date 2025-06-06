// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/entity"
	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ecsutil"
)

func TestTranslate(t *testing.T) {
	testCases := map[string]struct {
		input          map[string]interface{}
		mode           string
		kubernetesMode string
		envClusterName string
		inputTransform *entity.Transform
		want           *awsentity.Config
	}{
		"OnlyProfile": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{
							"hosted_in": "test",
						},
					},
				}},
			mode:           config.ModeEC2,
			kubernetesMode: config.ModeEKS,
			want: &awsentity.Config{
				ClusterName:    "test",
				KubernetesMode: config.ModeEKS,
				Platform:       config.ModeEC2,
			},
		},
		"KubernetesUnderLogs": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"cluster_name": "ci-logs",
						},
					},
				},
			},
			mode:           config.ModeEC2,
			kubernetesMode: config.ModeEKS,
			want: &awsentity.Config{
				ClusterName:    "ci-logs",
				KubernetesMode: config.ModeEKS,
				Platform:       config.ModeEC2,
			},
		},
		"EnvVar": {
			input:          map[string]interface{}{},
			mode:           config.ModeEC2,
			kubernetesMode: config.ModeEKS,
			envClusterName: "env-cluster",
			want: &awsentity.Config{
				ClusterName:    "env-cluster",
				KubernetesMode: config.ModeEKS,
				Platform:       config.ModeEC2,
			},
		},
		"AppSignalsPrecedence": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"app_signals": map[string]interface{}{
							"hosted_in": "test",
						},
						"kubernetes": map[string]interface{}{
							"cluster_name": "ci-logs",
						},
					},
				}},
			mode:           config.ModeEC2,
			kubernetesMode: config.ModeEKS,
			want: &awsentity.Config{
				ClusterName:    "test",
				KubernetesMode: config.ModeEKS,
				Platform:       config.ModeEC2,
			},
		},
		"KubernetesPrecedence": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"cluster_name": "ci-logs",
						},
					},
				},
			},
			mode:           config.ModeEC2,
			kubernetesMode: config.ModeEKS,
			envClusterName: "env-cluster",
			want: &awsentity.Config{
				ClusterName:    "ci-logs",
				KubernetesMode: config.ModeEKS,
				Platform:       config.ModeEC2,
			},
		},
		"ECS": {
			input: map[string]interface{}{},
			mode:  config.ModeECS,
			want:  nil,
		},
		"EC2WithTransform": {
			input: map[string]interface{}{},
			mode:  config.ModeEC2,
			inputTransform: &entity.Transform{
				KeyAttributes: []entity.KeyPair{
					{
						Key:   "Name",
						Value: "test-service",
					},
				},
				Attributes: []entity.KeyPair{
					{
						Key:   "AWS.ServiceNameSource",
						Value: "UserConfiguration",
					},
				},
			},
			want: &awsentity.Config{
				Platform: config.ModeEC2,
				TransformEntity: &entity.Transform{
					KeyAttributes: []entity.KeyPair{
						{
							Key:   "Name",
							Value: "test-service",
						},
					},
					Attributes: []entity.KeyPair{
						{
							Key:   "AWS.ServiceNameSource",
							Value: "UserConfiguration",
						},
					},
				},
			},
		},
		"KubernetesWithTransform": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": map[string]interface{}{
							"cluster_name": "k8s-cluster",
						},
					},
				},
			},
			mode:           config.ModeEC2,
			kubernetesMode: config.ModeEKS,
			inputTransform: &entity.Transform{
				KeyAttributes: []entity.KeyPair{
					{
						Key:   "Name",
						Value: "k8s-service",
					},
				},
				Attributes: []entity.KeyPair{
					{
						Key:   "AWS.ServiceNameSource",
						Value: "UserConfiguration",
					},
				},
			},
			want: &awsentity.Config{
				ClusterName:    "k8s-cluster",
				KubernetesMode: config.ModeEKS,
				Platform:       config.ModeEC2,
				TransformEntity: &entity.Transform{
					KeyAttributes: []entity.KeyPair{
						{
							Key:   "Name",
							Value: "k8s-service",
						},
					},
					Attributes: []entity.KeyPair{
						{
							Key:   "AWS.ServiceNameSource",
							Value: "UserConfiguration",
						},
					},
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			if testCase.mode == config.ModeECS {
				context.CurrentContext().SetRunInContainer(true)
				t.Setenv(config.RUN_IN_CONTAINER, config.RUN_IN_CONTAINER_TRUE)
				ecsutil.GetECSUtilSingleton().Region = "test"
			} else {
				ecsutil.GetECSUtilSingleton().Region = ""
				context.CurrentContext().SetMode(testCase.mode)
				context.CurrentContext().SetKubernetesMode(testCase.kubernetesMode)
			}
			if testCase.envClusterName != "" {
				t.Setenv("K8S_CLUSTER_NAME", testCase.envClusterName)
			} else {
				t.Setenv("K8S_CLUSTER_NAME", "")
			}
			var tt common.ComponentTranslator
			if testCase.inputTransform != nil {
				tt = NewTranslatorWithEntityTypeAndTransform("", "", false, testCase.inputTransform)
			} else {
				tt = NewTranslator()
			}
			assert.Equal(t, "awsentity", tt.ID().String())
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.NoError(t, err)
			if testCase.want == nil {
				assert.Nil(t, got)
			} else {
				assert.Equal(t, testCase.want, got)
			}
		})
	}
}
