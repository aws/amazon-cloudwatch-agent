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
	context.CurrentContext().SetKubernetesMode(config.ModeEKS)
	testCases := map[string]struct {
		input map[string]interface{}
		want  *awsentity.Config
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
			want: &awsentity.Config{
				ClusterName: "test",
				Mode:        config.ModeEKS,
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator()
			assert.Equal(t, "awsentity", tt.ID().String())
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}
