// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/receiver/adapter"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	testCases := map[string]struct {
		jsonCfg      map[string]interface{}
		cfgType      string
		key          string
		wantErr      error
		wantInterval time.Duration
	}{
		"WithoutKey": {
			jsonCfg: map[string]interface{}{},
			cfgType: "test",
			key:     "mem",
			wantErr: &common.MissingKeyError{Type: "telegraf_test", JsonKey: "mem"},
		},
		"WithoutInterval": {
			jsonCfg: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"cpu": map[string]interface{}{},
					},
				},
			},
			cfgType:      "test",
			key:          common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, "cpu"),
			wantInterval: time.Minute,
		},
		"WithValid": {
			jsonCfg: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"mem": map[string]interface{}{
							"measurement":                 []string{"mem_used_percent"},
							"metrics_collection_interval": "20s",
						},
					},
				},
			},
			cfgType:      "test",
			key:          common.ConfigKey(common.MetricsKey, common.MetricsCollectedKey, "mem"),
			wantInterval: 20 * time.Second,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.jsonCfg)
			tt := NewTranslator(testCase.cfgType, testCase.key)
			got, err := tt.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				cfg := got.(*adapter.Config)
				require.Equal(t, adapter.Type(testCase.cfgType), cfg.ID().Type())
				require.Equal(t, testCase.wantInterval, cfg.CollectionInterval)
			}
		})
	}
}

func TestGetMetricsCollectionInterval(t *testing.T) {
	sectionKeys := []string{"section", "backup"}
	testCases := map[string]struct {
		jsonCfg map[string]interface{}
		want    time.Duration
	}{
		"WithDefault": {
			jsonCfg: map[string]interface{}{},
			want:    time.Minute,
		},
		"WithoutSectionOverride": {
			jsonCfg: map[string]interface{}{
				"backup": map[string]interface{}{
					"metrics_collection_interval": 10,
				},
				"section": map[string]interface{}{},
			},
			want: 10 * time.Second,
		},
		"WithInvalidSectionOverride": {
			jsonCfg: map[string]interface{}{
				"backup": map[string]interface{}{
					"metrics_collection_interval": 10,
				},
				"section": map[string]interface{}{
					"metrics_collection_interval": "invalid",
				},
			},
			want: 10 * time.Second,
		},
		"WithSectionOverride": {
			jsonCfg: map[string]interface{}{
				"backup": map[string]interface{}{
					"metrics_collection_interval": 10,
				},
				"section": map[string]interface{}{
					"metrics_collection_interval": 120,
				},
			},
			want: 2 * time.Minute,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.jsonCfg)
			got := getMetricsCollectionInterval(conf, sectionKeys)
			require.Equal(t, testCase.want, got)
		})
	}
}
