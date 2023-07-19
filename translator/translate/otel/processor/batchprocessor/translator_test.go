// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package batchprocessor

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor/batchprocessor"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	testCases := map[string]struct {
		translator common.Translator[component.Config]
		input      map[string]interface{}
		want       *batchprocessor.Config
		wantErr    error
	}{
		"DefaultMetricsSection": {
			translator: NewTranslatorWithNameAndSection("test", common.MetricsKey),
			input: map[string]interface{}{
				"metrics": map[string]interface{}{},
			},
			want: &batchprocessor.Config{
				Timeout:          60 * time.Second,
				SendBatchSize:    8192,
				SendBatchMaxSize: 0,
			},
		},
		"DefaultLogsSection": {
			translator: NewTranslatorWithNameAndSection("test", common.LogsKey),
			input: map[string]interface{}{
				"logs": map[string]interface{}{},
			},
			want: &batchprocessor.Config{
				Timeout:          5 * time.Second,
				SendBatchSize:    8192,
				SendBatchMaxSize: 0,
			},
		},
		"DefaultNotConfiguredSection": {
			translator: NewTranslatorWithNameAndSection("test", common.TracesKey),
			input: map[string]interface{}{
				"traces": map[string]interface{}{},
			},
			wantErr: errors.New("default force_flush_interval not defined for traces"),
		},
		"OverrideForceFlushIntervalMetricsSection": {
			translator: NewTranslatorWithNameAndSection("test", common.MetricsKey),
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"force_flush_interval": 30,
				},
			},
			want: &batchprocessor.Config{
				Timeout:          30 * time.Second,
				SendBatchSize:    8192,
				SendBatchMaxSize: 0,
			},
		},
		"OverrideForceFlushIntervalLogsSection": {
			translator: NewTranslatorWithNameAndSection("test", common.LogsKey),
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"force_flush_interval": 30,
				},
			},
			want: &batchprocessor.Config{
				Timeout:          30 * time.Second,
				SendBatchSize:    8192,
				SendBatchMaxSize: 0,
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tc.input)
			got, err := tc.translator.Translate(conf)
			require.Equal(t, tc.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*batchprocessor.Config)
				require.True(t, ok)
				require.Equal(t, tc.want.Timeout, gotCfg.Timeout)
				require.Equal(t, tc.want.SendBatchSize, gotCfg.SendBatchSize)
				require.Equal(t, tc.want.SendBatchMaxSize, gotCfg.SendBatchMaxSize)
			}
		})
	}
}
