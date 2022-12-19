// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package containerinsights

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/private-amazon-cloudwatch-agent-staging/internal/util/collections"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	type want struct {
		pipelineType string
		receivers    []string
		processors   []string
		exporters    []string
	}
	cit := NewTranslator()
	require.EqualValues(t, "containerinsights", cit.Type())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *want
		wantErr error
	}{
		"WithoutECSKey": {
			input:   map[string]interface{}{},
			wantErr: &common.MissingKeyError{Type: cit.Type(), JsonKey: fmt.Sprint(ecsKey, " or ", eksKey)},
		},
		"WithECSKey": {
			input: map[string]interface{}{
				"logs": map[string]interface{}{
					"metrics_collected": map[string]interface{}{
						"ecs": nil,
					},
				},
			},
			want: &want{
				pipelineType: "metrics/containerinsights",
				receivers:    []string{"awscontainerinsightreceiver"},
				processors:   []string{"batch/containerinsights"},
				exporters:    []string{"awsemf/containerinsights"},
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
				require.EqualValues(t, testCase.want.pipelineType, got.Key.String())
				require.Equal(t, testCase.want.receivers, collections.MapSlice(got.Value.Receivers, toString))
				require.Equal(t, testCase.want.processors, collections.MapSlice(got.Value.Processors, toString))
				require.Equal(t, testCase.want.exporters, collections.MapSlice(got.Value.Exporters, toString))
			}
		})
	}
}

func toString(id component.ID) string {
	return id.String()
}
