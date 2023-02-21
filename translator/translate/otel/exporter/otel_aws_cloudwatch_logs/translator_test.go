// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otel_aws_cloudwatch_logs

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/pipeline/emf_logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"
	"go.opentelemetry.io/collector/component"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	legacytranslator "github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

func TestTranslator(t *testing.T) {
	tt := NewTranslator()
	require.EqualValues(t, "awscloudwatchlogs", tt.Type())
	testCases := map[string]struct {
		env               map[string]string
		input             map[string]interface{}
		translatorOptions common.TranslatorOptions
		want              awscloudwatchlogsexporter.Config
		wantErr           error
	}{
		"Emf": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"emf": map[string]interface{}{},
					},
					"log_stream_name": "same random stream",
				},
			},
			translatorOptions: common.TranslatorOptions{PipelineId: component.NewIDWithName("awscloudwatchlogs", emf_logs.PipelineName)},
			want: awscloudwatchlogsexporter.Config{
				LogGroupName:  "emf/logs/default",
				LogStreamName: "same random stream",
				RawLog:        true,
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf, testCase.translatorOptions)
			require.Equal(t, testCase.wantErr, err)
			require.Truef(t, legacytranslator.IsTranslateSuccess(), "Error in legacy translation rules: %v", legacytranslator.ErrorMessages)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*awscloudwatchlogsexporter.Config)
				require.True(t, ok)
				require.Equal(t, testCase.want.LogGroupName, gotCfg.LogGroupName)
				require.Equal(t, testCase.want.LogStreamName, gotCfg.LogStreamName)
				require.Equal(t, testCase.want.RawLog, gotCfg.RawLog)
				require.Equal(t, testCase.want.Region, gotCfg.Region)
			}
		})
	}
}
