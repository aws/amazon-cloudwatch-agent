// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsightsjmx

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
	require.EqualValues(t, "metrics/containerinsightsjmx", cit.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *want
		wantErr error
	}{
		"WithoutECSOrKubernetesKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{ID: cit.ID(), JsonKey: fmt.Sprint(eksKey)},
		},
		"WithKubernetesKey": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"kubernetes": nil,
					},
				},
			},
			want: &want{
				pipelineType: "metrics/containerinsightsjmx",
				receivers:    []string{"otlp/jmx"},
				processors:   []string{"filter/containerinsightsjmx", "resourcedetection", "resource/jmxResource", "transform/containerinsightsjmx", "metricstransform/containerinsightsjmx"},
				exporters:    []string{"debug", "awsemf/jmx"},
				extensions:   []string{},
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
