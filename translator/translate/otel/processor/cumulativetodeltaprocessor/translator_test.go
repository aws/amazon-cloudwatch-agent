// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cumulativetodeltaprocessor

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	cdpTranslator := NewTranslator(common.WithName("test"), WithDefaultKeys())
	require.EqualValues(t, "cumulativetodelta/test", cdpTranslator.ID().String())
	testCases := map[string]struct {
		input   map[string]any
		want    map[string]any
		wantErr error
	}{
		"GenerateDeltaProcessorConfigWithCPU": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"cpu": map[string]any{},
					},
				},
			},
			wantErr: &common.MissingKeyError{ID: cdpTranslator.ID(), JsonKey: fmt.Sprint(diskioKey, " or ", netKey, " or ", otlpKey, " or ", otlpEmfKey)},
		},
		"GenerateDeltaProcessorConfigWithNet": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"net": map[string]any{},
					},
				},
			},
			want: map[string]any{
				"initial_value": "drop",
			},
		},
		"GenerateDeltaProcessorConfigWithDiskIO": {
			input: map[string]any{
				"metrics": map[string]any{
					"metrics_collected": map[string]any{
						"diskio": map[string]any{},
					},
				},
			},
			want: map[string]any{
				"exclude": map[string]any{
					"match_type": "strict",
					"metrics":    []string{"iops_in_progress", "diskio_iops_in_progress", "diskio_ebs_volume_queue_length"},
				},
				"initial_value": "drop",
			},
		},
	}
	factory := cumulativetodeltaprocessor.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := cdpTranslator.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*cumulativetodeltaprocessor.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				wantConf := confmap.NewFromStringMap(testCase.want)
				require.NoError(t, wantConf.Unmarshal(&wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}
