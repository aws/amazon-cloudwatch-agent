// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sattributesprocessor

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

func TestTranslate(t *testing.T) {
	testCases := map[string]struct {
		input          map[string]interface{}
		mode           string
		kubernetesMode string
		workloadType   string
		want           *k8sattributesprocessor.Config
		wantErr        error
	}{
		"DaemonSet": {
			input:          map[string]interface{}{},
			mode:           config.ModeEC2,
			kubernetesMode: config.ModeEKS,
			workloadType:   config.DaemonSet,
			want: &k8sattributesprocessor.Config{
				Association: []k8sattributesprocessor.PodAssociationConfig{
					{
						Sources: []k8sattributesprocessor.PodAssociationSourceConfig{
							{
								From: "connection",
							},
						},
					},
				},
				Extract: k8sattributesprocessor.ExtractConfig{
					Metadata: []string{"k8s.namespace.name", "k8s.pod.name", "k8s.replicaset.name", "k8s.deployment.name", "k8s.daemonset.name", "k8s.statefulset.name", "k8s.cronjob.name", "k8s.job.name", "k8s.node.name"},
				},
				Filter: k8sattributesprocessor.FilterConfig{
					NodeFromEnvVar: "K8S_NODE_NAME",
				},
			},
		},
		"Deployment": {
			input:          map[string]interface{}{},
			mode:           config.ModeEC2,
			kubernetesMode: config.ModeEKS,
			workloadType:   config.Deployment,
			want: &k8sattributesprocessor.Config{
				Association: []k8sattributesprocessor.PodAssociationConfig{
					{
						Sources: []k8sattributesprocessor.PodAssociationSourceConfig{
							{
								From: "connection",
							},
						},
					},
				},
				Extract: k8sattributesprocessor.ExtractConfig{
					Metadata: []string{"k8s.namespace.name", "k8s.pod.name", "k8s.replicaset.name", "k8s.deployment.name", "k8s.daemonset.name", "k8s.statefulset.name", "k8s.cronjob.name", "k8s.job.name", "k8s.node.name"},
				},
				Filter: k8sattributesprocessor.FilterConfig{
					NodeFromEnvVar: "",
				},
			},
		},
		"StatefulSet": {
			input:          map[string]interface{}{},
			mode:           config.ModeEC2,
			kubernetesMode: config.ModeEKS,
			workloadType:   config.StatefulSet,
			want: &k8sattributesprocessor.Config{
				Association: []k8sattributesprocessor.PodAssociationConfig{
					{
						Sources: []k8sattributesprocessor.PodAssociationSourceConfig{
							{
								From: "connection",
							},
						},
					},
				},
				Extract: k8sattributesprocessor.ExtractConfig{
					Metadata: []string{"k8s.namespace.name", "k8s.pod.name", "k8s.replicaset.name", "k8s.deployment.name", "k8s.daemonset.name", "k8s.statefulset.name", "k8s.cronjob.name", "k8s.job.name", "k8s.node.name"},
				},
				Filter: k8sattributesprocessor.FilterConfig{
					NodeFromEnvVar: "",
				},
			},
		},
		"NotKubernetes": {
			input:          map[string]interface{}{},
			mode:           config.ModeEC2,
			kubernetesMode: "",
			workloadType:   "Unknown",
			wantErr:        fmt.Errorf("k8sattributesprocessor is not supported in this context"),
		},
		"Unknown": {
			input:          map[string]interface{}{},
			mode:           config.ModeEC2,
			kubernetesMode: config.ModeEKS,
			workloadType:   "Unknown",
			wantErr:        fmt.Errorf("k8sattributesprocessor is not supported for workload type: "),
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			context.CurrentContext().SetMode(testCase.mode)
			context.CurrentContext().SetKubernetesMode(testCase.kubernetesMode)
			context.CurrentContext().SetWorkloadType(testCase.workloadType)
			tt := NewTranslatorWithName("")
			assert.Equal(t, "k8sattributes", tt.ID().String())
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)

			if testCase.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, testCase.wantErr.Error(), err.Error())
				return
			}

			assert.NoError(t, err)
			if testCase.want == nil {
				assert.Nil(t, got)
			} else {
				expect := got.(*k8sattributesprocessor.Config)
				assert.Equal(t, testCase.want.Association, expect.Association)
				assert.Equal(t, testCase.want.Extract, expect.Extract)
				assert.Equal(t, testCase.want.Filter, expect.Filter)
			}
		})
	}
}
