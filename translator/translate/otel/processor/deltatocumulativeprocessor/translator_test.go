// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package deltatocumulativeprocessor

import (
	"math"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	dcpTranslator := NewTranslator(common.WithName("test"))
	require.EqualValues(t, "deltatocumulative/test", dcpTranslator.ID().String())
	testCases := map[string]struct {
		input   map[string]any
		want    map[string]any
		wantErr error
	}{
		"EmptyConfig": {
			input: map[string]any{},
			want: map[string]any{
				"max_stale":   1209600000000000, // 2 weeks, in minutes
				"max_streams": math.MaxInt64,
			},
		},
	}
	factory := deltatocumulativeprocessor.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := dcpTranslator.Translate(conf)
			require.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*deltatocumulativeprocessor.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig()
				wantConf := confmap.NewFromStringMap(testCase.want)
				require.NoError(t, wantConf.Unmarshal(&wantCfg))
				assert.Equal(t, wantCfg, gotCfg)
			}
		})
	}
}
