// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otel_aws_cloudwatch_logs

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	legacytranslator "github.com/aws/amazon-cloudwatch-agent/translator"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	tt := NewTranslatorWithName(common.PipelineNameEmfLogs)
	require.EqualValues(t, "awscloudwatchlogs/emf_logs", tt.ID().String())
	testCases := map[string]struct {
		env     map[string]string
		input   map[string]interface{}
		want    awscloudwatchlogsexporter.Config
		wantErr error
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
			got, err := tt.Translate(conf)
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
