// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package adapter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/receiver/adapter"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	testCases := map[string]struct {
		input        map[string]interface{}
		cfgType      string
		cfgKey       string
		wantErr      error
		wantInterval time.Duration
	}{
		"WithoutKeyInConfig": {
			input:   map[string]interface{}{},
			cfgType: "test",
			cfgKey:  "mem",
			wantErr: &common.MissingKeyError{ID: component.NewID("telegraf_test"), JsonKey: "mem"},
		},
		"WithoutIntervalInSection": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"cpu": map[string]interface{}{},
					},
				},
			},
			cfgType:      "test",
			cfgKey:       "metrics::metrics_collected::cpu",
			wantInterval: time.Minute,
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
			cfgType:      "test",
			cfgKey:       "metrics::metrics_collected::mem",
			wantInterval: 20 * time.Second,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			tt := NewTranslator(testCase.cfgType, testCase.cfgKey, time.Minute)
			got, err := tt.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*adapter.Config)
				require.True(t, ok)
				require.Equal(t, adapter.Type(testCase.cfgType), tt.ID().Type())
				require.Equal(t, testCase.wantInterval, gotCfg.CollectionInterval)
			}
		})
	}
}
