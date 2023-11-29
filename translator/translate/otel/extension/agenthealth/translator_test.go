// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
)

func TestTranslate(t *testing.T) {
	operations := []string{OperationPutLogEvents}
	testCases := map[string]struct {
		input          map[string]interface{}
		isEnvUsageData bool
		want           *agenthealth.Config
	}{
		"WithUsageData/NotInConfig": {
			input:          map[string]interface{}{"agent": map[string]interface{}{}},
			isEnvUsageData: true,
			want: &agenthealth.Config{
				IsUsageDataEnabled: true,
				Stats: agent.StatsConfig{
					Operations: operations,
				},
			},
		},
		"WithUsageData/FalseInConfig": {
			input:          map[string]interface{}{"agent": map[string]interface{}{"usage_data": false}},
			isEnvUsageData: true,
			want: &agenthealth.Config{
				IsUsageDataEnabled: false,
				Stats: agent.StatsConfig{
					Operations: operations,
				},
			},
		},
		"WithUsageData/FalseInEnv": {
			input:          map[string]interface{}{"agent": map[string]interface{}{"usage_data": true}},
			isEnvUsageData: false,
			want: &agenthealth.Config{
				IsUsageDataEnabled: false,
				Stats: agent.StatsConfig{
					Operations: operations,
				},
			},
		},
		"WithUsageData/BothTrue": {
			input:          map[string]interface{}{"agent": map[string]interface{}{"usage_data": true}},
			isEnvUsageData: true,
			want: &agenthealth.Config{
				IsUsageDataEnabled: true,
				Stats: agent.StatsConfig{
					Operations: operations,
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator("test", operations).(*translator)
			assert.Equal(t, "agenthealth/test", tt.ID().String())
			tt.isUsageDataEnabled = testCase.isEnvUsageData
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}
