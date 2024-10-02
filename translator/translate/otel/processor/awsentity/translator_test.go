// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsentity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/plugins/processors/awsentity"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
)

func TestTranslate(t *testing.T) {
	testCases := map[string]struct {
		input          map[string]interface{}
		mode           string
		kubernetesMode string
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
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			context.CurrentContext().SetMode(testCase.mode)
			context.CurrentContext().SetKubernetesMode(testCase.kubernetesMode)
			tt := NewTranslator()
			assert.Equal(t, "awsentity", tt.ID().String())
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}
