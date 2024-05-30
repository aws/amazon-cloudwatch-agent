// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/hash"
	"github.com/aws/amazon-cloudwatch-agent/receiver/adapter"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	telegrafTestType, _ := component.NewType("telegraf_test")
	testCases := map[string]struct {
		input             map[string]interface{}
		cfgName           string
		cfgType           string
		cfgKey            string
		cfgPreferInterval time.Duration
		wantErr           error
		wantInterval      time.Duration
	}{
		"WithoutKeyInConfig": {
			input:   map[string]interface{}{},
			cfgName: "",
			cfgType: "test",
			cfgKey:  "mem",
			wantErr: &common.MissingKeyError{ID: component.NewID(telegrafTestType), JsonKey: "mem"},
		},
		"WithoutIntervalInSection": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"cpu": map[string]interface{}{},
					},
				},
			},
			cfgName:           "",
			cfgType:           "test",
			cfgKey:            "metrics::metrics_collected::cpu",
			cfgPreferInterval: time.Duration(0),
			wantInterval:      time.Minute,
		},
		"WithPreferInterval": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"mem": map[string]interface{}{
							"measurement":                 []string{"mem_used_percent"},
							"metrics_collection_interval": "5s",
						},
					},
				},
			},
			cfgName:           "",
			cfgType:           "test",
			cfgKey:            "metrics::metrics_collected::mem",
			cfgPreferInterval: 15 * time.Second,
			wantInterval:      15 * time.Second,
		},
		"WithValidConfig": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"mem": map[string]interface{}{
							"measurement":                 []string{"mem_used_percent"},
							"metrics_collection_interval": "20s",
						},
					},
				},
			},
			cfgName:           "",
			cfgType:           "test",
			cfgKey:            "metrics::metrics_collected::mem",
			cfgPreferInterval: time.Duration(0),
			wantInterval:      20 * time.Second,
		},
		"WithWindowsConfig": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"LogicalDisk": map[string]interface{}{
							"measurement":                 []string{"% Free Space"},
							"metrics_collection_interval": 10,
						},
					},
				},
			},
			cfgName:           "LogicalDisk",
			cfgType:           "test",
			cfgKey:            "metrics::metrics_collected::LogicalDisk",
			cfgPreferInterval: time.Duration(0),
			wantInterval:      10 * time.Second,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			tt := NewTranslatorWithName(testCase.cfgName, testCase.cfgType, testCase.cfgKey, testCase.cfgPreferInterval, time.Minute)
			got, err := tt.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*adapter.Config)
				require.True(t, ok)
				require.Equal(t, hash.HashName(testCase.cfgName), tt.ID().Name())
				require.Equal(t, adapter.Type(testCase.cfgType), tt.ID().Type())
				require.Equal(t, testCase.wantInterval, gotCfg.CollectionInterval)
				require.Equal(t, testCase.cfgName, gotCfg.AliasName)
			}
		})
	}
}
