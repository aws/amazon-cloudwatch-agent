// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package rollupprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/metric"
	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/processor/rollupprocessor"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	rpt := NewTranslator()
	require.EqualValues(t, "rollup", rpt.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *rollupprocessor.Config
		wantErr error
	}{
		"WithMissingKey": {
			input: map[string]interface{}{"metrics": map[string]interface{}{}},
			wantErr: &common.MissingKeyError{
				ID:      rpt.ID(),
				JsonKey: common.MetricsAggregationDimensionsKey,
			},
		},
		"WithOnlyAggregationDimensions": {
			input: map[string]interface{}{
				"metrics": map[string]interface{}{
					"aggregation_dimensions": []interface{}{[]interface{}{"d1", "d2"}},
				},
			},
			want: &rollupprocessor.Config{
				AttributeGroups: [][]string{{"d1", "d2"}},
				CacheSize:       1000,
			},
		},
		"WithFull": {
			input: testutil.GetJson(t, filepath.Join("..", "..", "common", "testdata", "config.json")),
			want: &rollupprocessor.Config{
				AttributeGroups: [][]string{{"ImageId"}, {"InstanceId", "InstanceType"}, {"d1"}, {}},
				DropOriginal:    []string{"CPU_USAGE_IDLE", metric.DecorateMetricName("cpu", "time_active")},
				CacheSize:       1000,
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := rpt.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if testCase.want != nil {
				require.NoError(t, err)
				gotCfg, ok := got.(*rollupprocessor.Config)
				require.True(t, ok)
				assert.Equal(t, testCase.want.AttributeGroups, gotCfg.AttributeGroups)
				assert.Equal(t, testCase.want.DropOriginal, gotCfg.DropOriginal)
			}
		})
	}
}
