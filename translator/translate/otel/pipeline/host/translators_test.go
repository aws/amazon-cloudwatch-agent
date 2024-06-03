// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package host

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/collections"
	translatorcontext "github.com/aws/amazon-cloudwatch-agent/translator"
	_ "github.com/aws/amazon-cloudwatch-agent/translator/registerrules"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslators(t *testing.T) {
	type want struct {
		receivers []string
		exporters []string
	}
	testCases := map[string]struct {
		input map[string]interface{}
		want  map[string]want
	}{
		"WithEmpty": {
			input: map[string]interface{}{},
			want:  map[string]want{},
		},
		"WithMinimal": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"cpu": map[string]interface{}{},
					},
				},
			},
			want: map[string]want{
				"metrics/host": {
					receivers: []string{"telegraf_cpu"},
					exporters: []string{"awscloudwatch"},
				},
			},
		},
		"WithAMPDestination": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_destinations": map[string]interface{}{
						"amp": map[string]interface{}{
							"workspace_id": "ws-12345",
						},
					},
					"metrics_collected": map[string]interface{}{
						"cpu": map[string]interface{}{},
					},
				},
			},
			want: map[string]want{
				"metrics/host_other": {
					receivers: []string{"telegraf_cpu"},
					exporters: []string{"prometheusremotewrite/amp"},
				},
			},
		},
		"WithAMPAndCloudWatchDestinations": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_destinations": map[string]interface{}{
						"amp": map[string]interface{}{
							"workspace_id": "ws-12345",
						},
						"cloudwatch": map[string]interface{}{},
					},
					"metrics_collected": map[string]interface{}{
						"cpu": map[string]interface{}{},
					},
				},
			},
			want: map[string]want{
				"metrics/host": {
					receivers: []string{"telegraf_cpu"},
					exporters: []string{"awscloudwatch"},
				},
				"metrics/host_other": {
					receivers: []string{"telegraf_cpu"},
					exporters: []string{"prometheusremotewrite/amp"},
				},
			},
		},
		"WithDeltaMetrics": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"metrics_destinations": map[string]interface{}{
						"amp": map[string]interface{}{
							"workspace_id": "ws-12345",
						},
						"cloudwatch": map[string]interface{}{},
					},
					"metrics_collected": map[string]interface{}{
						"net": map[string]interface{}{},
					},
				},
			},
			want: map[string]want{
				"metrics/hostDeltaMetrics": {
					receivers: []string{"telegraf_net"},
					exporters: []string{"awscloudwatch"},
				},
				"metrics/host_other": {
					receivers: []string{"telegraf_net"},
					exporters: []string{"prometheusremotewrite/amp"},
				},
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			translatorcontext.SetTargetPlatform("linux")
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := NewTranslators(conf, "linux")
			require.NoError(t, err)
			if testCase.want == nil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, len(testCase.want), got.Len())
				got.Range(func(tr common.Translator[*common.ComponentTranslators]) {
					w, ok := testCase.want[tr.ID().String()]
					require.True(t, ok)
					assert.Equal(t, w.receivers, collections.MapSlice(tr.(*translator).receivers.Keys(), component.ID.String))
					assert.Equal(t, w.exporters, collections.MapSlice(tr.(*translator).exporters.Keys(), component.ID.String))
				})
			}
		})
	}
}

func TestTranslatorsError(t *testing.T) {
	got, err := NewTranslators(confmap.New(), "invalid")
	assert.Error(t, err)
	assert.Nil(t, got)
}
