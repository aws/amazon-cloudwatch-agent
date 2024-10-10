// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package prometheus

import (
	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTranslator(t *testing.T) {
	//factory := prometheusreceiver.NewFactory()
	testCases := map[string]struct {
		input   map[string]any
		index   int
		wantID  string
		want    string
		wantErr error
	}{
		"WithCompleteConfig": {
			input:  testutil.GetJson(t, filepath.Join("testdata", "config.json")),
			index:  -1,
			wantID: "prometheus",
			want:   filepath.Join("testdata", "config.yaml"),
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			tt := NewTranslator(WithIndex(testCase.index))
			assert.EqualValues(t, testCase.wantID, tt.ID().String())
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*prometheusreceiver.Config)
				require.True(t, ok)
				wantCfg := &prometheusConfig{}
				content, err := os.ReadFile(testCase.want)
				assert.NoError(t, err)
				err = yaml.Unmarshal(content, &wantCfg)
				assert.NoError(t, err)
				assert.Equal(t, wantCfg.ScrapeConfigs, gotCfg.PrometheusConfig.ScrapeConfigs)
				assert.Equal(t, wantCfg.GlobalConfig, gotCfg.PrometheusConfig.GlobalConfig)
				//assert.Equal(t, wantCfg.TargetAllocator, gotCfg.TargetAllocator)
			}
		})
	}
}
