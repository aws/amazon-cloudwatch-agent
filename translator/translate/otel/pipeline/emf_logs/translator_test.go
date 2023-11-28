// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emf_logs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	type want struct {
		pipelineType string
		receivers    []string
		processors   []string
		exporters    []string
		extensions   []string
	}
	cit := NewTranslator()
	require.EqualValues(t, "logs/emf_logs", cit.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *want
		wantErr error
	}{
		"WithoutEmfKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{ID: cit.ID(), JsonKey: fmt.Sprint(emfKey)},
		},
		"WithEmfKey": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"emf": nil,
					},
				},
			},
			want: &want{
				pipelineType: "logs/emf_logs",
				receivers:    []string{"tcplog/emf_logs", "udplog/emf_logs"},
				processors:   []string{"batch/emf_logs"},
				exporters:    []string{"awscloudwatchlogs/emf_logs"},
				extensions:   []string{"agenthealth/logs"},
			},
		},
		"WithStructuredLogKey": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"structuredlog": nil,
					},
				},
			},
			want: &want{
				pipelineType: "logs/emf_logs",
				receivers:    []string{"tcplog/emf_logs", "udplog/emf_logs"},
				processors:   []string{"batch/emf_logs"},
				exporters:    []string{"awscloudwatchlogs/emf_logs"},
				extensions:   []string{"agenthealth/logs"},
			},
		},
		"WithUdpServiceAddress": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"emf": map[string]interface{}{
							"service_address": "udp:1000",
						},
					},
				},
			},
			want: &want{
				pipelineType: "logs/emf_logs",
				receivers:    []string{"udplog/emf_logs"},
				processors:   []string{"batch/emf_logs"},
				exporters:    []string{"awscloudwatchlogs/emf_logs"},
				extensions:   []string{"agenthealth/logs"},
			},
		},
		"WithTcpServiceAddress": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"emf": map[string]interface{}{
							"service_address": "tcp:1000",
						},
					},
				},
			},
			want: &want{
				pipelineType: "logs/emf_logs",
				receivers:    []string{"tcplog/emf_logs"},
				processors:   []string{"batch/emf_logs"},
				exporters:    []string{"awscloudwatchlogs/emf_logs"},
				extensions:   []string{"agenthealth/logs"},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := cit.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if testCase.want == nil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, testCase.want.receivers, collections.MapSlice(got.Receivers.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.processors, collections.MapSlice(got.Processors.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.exporters, collections.MapSlice(got.Exporters.Keys(), component.ID.String))
				assert.Equal(t, testCase.want.extensions, collections.MapSlice(got.Extensions.Keys(), component.ID.String))
			}
		})
	}
}
