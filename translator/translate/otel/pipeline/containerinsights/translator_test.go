package containerinsights

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
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
		input map[string]interface{}
		want  *want
	}{
		"WithoutKey": {
			input: map[string]interface{}{},
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
			got, _ := cit.Translate(conf)
			if testCase.want == nil {
				require.Nil(t, got)
			} else {
				require.EqualValues(t, testCase.want.pipelineType, got.Key.String())
				require.Equal(t, testCase.want.receivers, toStringSlice(got.Value.Receivers))
				require.Equal(t, testCase.want.processors, toStringSlice(got.Value.Processors))
				require.Equal(t, testCase.want.exporters, toStringSlice(got.Value.Exporters))
			}
		})
	}
}

func toStringSlice(ids []config.ComponentID) []string {
	var values []string
	for _, id := range ids {
		values = append(values, id.String())
	}
	return values
}
