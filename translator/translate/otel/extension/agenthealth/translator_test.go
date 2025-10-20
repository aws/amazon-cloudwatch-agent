// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agenthealth

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/handler/stats/agent"
	"github.com/aws/amazon-cloudwatch-agent/extension/agenthealth/metadata"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
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
		input          map[string]any
		isEnvUsageData bool
		want           *agenthealth.Config
	}{
		"WithUsageData/NotInConfig": {
			input:          map[string]any{"agent": map[string]any{}},
			isEnvUsageData: true,
			want: &agenthealth.Config{
				IsUsageDataEnabled: true,
				Stats: &agent.StatsConfig{
					Operations: operations,
					UsageFlags: usageFlags,
				},
			},
		},
		"WithUsageData/FalseInConfig": {
			input:          map[string]any{"agent": map[string]any{"usage_data": false}},
			isEnvUsageData: true,
			want: &agenthealth.Config{
				IsUsageDataEnabled: false,
				Stats: &agent.StatsConfig{
					Operations: operations,
					UsageFlags: usageFlags,
				},
			},
		},
		"WithUsageData/FalseInEnv": {
			input:          map[string]any{"agent": map[string]any{"usage_data": true}},
			isEnvUsageData: false,
			want: &agenthealth.Config{
				IsUsageDataEnabled: false,
				Stats: &agent.StatsConfig{
					Operations: operations,
					UsageFlags: usageFlags,
				},
			},
		},
		"WithUsageData/BothTrue": {
			input:          map[string]any{"agent": map[string]any{"usage_data": true}},
			isEnvUsageData: true,
			want: &agenthealth.Config{
				IsUsageDataEnabled: true,
				Stats: &agent.StatsConfig{
					Operations: operations,
					UsageFlags: usageFlags,
				},
			},
		},
		"WithUsageMetadata/OnlyUnsupported": {
			input: map[string]any{
				"agent": map[string]any{
					"usage_data": true,
					"usage_metadata": []any{map[string]any{
						"unsupported_key": "unsupported_value",
					},
					},
				},
			},
			isEnvUsageData: true,
			want: &agenthealth.Config{
				IsUsageDataEnabled: true,
				Stats: &agent.StatsConfig{
					Operations: operations,
					UsageFlags: usageFlags,
				},
			},
		},
		"WithUsageMetadata/Mixed": {
			input: map[string]any{
				"agent": map[string]any{
					"usage_data": true,
					"usage_metadata": []any{
						map[string]any{
							"ObservabilitySolution": "jvm",
							"test":                  "value",
						},
						map[string]any{
							"unsupported_key": "unsupported_value",
						},
					},
				},
			},
			isEnvUsageData: true,
			want: &agenthealth.Config{
				IsUsageDataEnabled: true,
				Stats: &agent.StatsConfig{
					Operations: operations,
					UsageFlags: usageFlags,
				},
				UsageMetadata: []metadata.Metadata{"obs_jvm"},
			},
		},
		"WithUsageMetadata/Supported": {
			input:          testutil.GetJson(t, filepath.Join("testdata", "config.json")),
			isEnvUsageData: true,
			want: &agenthealth.Config{
				IsUsageDataEnabled: true,
				Stats: &agent.StatsConfig{
					Operations: operations,
					UsageFlags: usageFlags,
				},
				UsageMetadata: []metadata.Metadata{"obs_jvm"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator(LogsName, operations).(*translator)
			assert.Equal(t, "agenthealth/logs", tt.ID().String())
			tt.isUsageDataEnabled = testCase.isEnvUsageData
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}
