// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	translateagent "github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

func TestTranslate(t *testing.T) {
	context.CurrentContext().SetMode(config.ModeEC2)
	translateagent.Global_Config.RegionType = config.RegionTypeNotFound
	operations := []string{OperationPutLogEvents}
	usageFlags := map[agent.Flag]any{
		agent.FlagMode:       config.ShortModeEC2,
		agent.FlagRegionType: config.RegionTypeNotFound,
	}
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
					UsageFlags: usageFlags,
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
					UsageFlags: usageFlags,
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
					UsageFlags: usageFlags,
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
					UsageFlags: usageFlags,
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			testType, _ := component.NewType("test")
			tt := NewTranslator(testType, operations).(*translator)
			assert.Equal(t, "agenthealth/test", tt.ID().String())
			tt.isUsageDataEnabled = testCase.isEnvUsageData
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}
