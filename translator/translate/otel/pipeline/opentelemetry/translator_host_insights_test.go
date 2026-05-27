// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestHostInsightsTranslator(t *testing.T) {
	tt := NewHostInsightsTranslator()
	assert.EqualValues(t, "metrics/host_insights", tt.ID().String())

	testCases := map[string]struct {
		input   map[string]interface{}
		wantErr error
	}{
		"WithNilConf": {
			input:   nil,
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: hostInsightsKey},
		},
		"WithoutHostInsightsKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{ID: tt.ID(), JsonKey: hostInsightsKey},
		},
		"WithHostInsightsKey": {
			input: map[string]interface{}{
				"opentelemetry": map[string]interface{}{
					"collect": map[string]interface{}{
						"host_insights": map[string]interface{}{},
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			agent.Global_Config.Region = "us-east-1"
			var conf *confmap.Conf
			if tc.input != nil {
				conf = confmap.NewFromStringMap(tc.input)
			}
			got, err := tt.Translate(conf)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.Equal(t, tc.wantErr, err)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, 1, got.Receivers.Len())
				assert.Equal(t, 1, got.Processors.Len())
				assert.Equal(t, 1, got.Exporters.Len())
				assert.Equal(t, 1, got.Extensions.Len())
				assert.Equal(t, "systemmetrics", got.Receivers.Keys()[0].String())
				assert.Equal(t, "resourcedetection", got.Processors.Keys()[0].String())
				assert.Equal(t, "otlphttp/host_insights", got.Exporters.Keys()[0].String())
				assert.Equal(t, "sigv4auth/monitoring", got.Extensions.Keys()[0].String())
			}
		})
	}
}

func TestHostInsightsTranslatorEmptyRegion(t *testing.T) {
	agent.Global_Config.Region = ""
	tt := NewHostInsightsTranslator()
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"opentelemetry": map[string]interface{}{
			"collect": map[string]interface{}{
				"host_insights": map[string]interface{}{},
			},
		},
	})
	got, err := tt.Translate(conf)
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "region is required")
}

func TestServiceEndpoint(t *testing.T) {
	testCases := map[string]struct {
		service string
		region  string
		path    string
		want    string
	}{
		"StandardPartition": {
			service: "monitoring",
			region:  "us-east-1",
			path:    "/v1/metrics",
			want:    "https://monitoring.us-east-1.amazonaws.com/v1/metrics",
		},
		"ChinaPartition": {
			service: "monitoring",
			region:  "cn-north-1",
			path:    "/v1/metrics",
			want:    "https://monitoring.cn-north-1.amazonaws.com.cn/v1/metrics",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := serviceEndpoint(tc.service, tc.region, tc.path)
			assert.Equal(t, tc.want, got)
		})
	}
}
