// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsefa

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsefareceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/internal/util/testutil"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslator(t *testing.T) {
	tt := NewTranslator()
	assert.EqualValues(t, "awsefareceiver", tt.ID().String())
	testCases := map[string]struct {
		input   map[string]interface{}
		want    *confmap.Conf
		wantErr error
	}{
		"WithMissingKey": {
			input: map[string]interface{}{"logs": map[string]interface{}{}},
			wantErr: &common.MissingKeyError{
				ID:      tt.ID(),
				JsonKey: baseKey,
			},
		},
		"WithEmptyConfig": {
			input:   testutil.GetJson(t, filepath.Join("testdata", "emptyConfig.json")),
			wantErr: fmt.Errorf("measurement is required for efa receiver (%s)", tt.ID()),
		},
		"WithMeasurements": {
			input: testutil.GetJson(t, filepath.Join("testdata", "config.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "config.yaml")),
		},
		"WithNonPrefixedMeasurements": {
			input: testutil.GetJson(t, filepath.Join("testdata", "nonPrefixedConfig.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "nonPrefixedConfig.yaml")),
		},
		"WithEmptyMeasurements": {
			input: testutil.GetJson(t, filepath.Join("testdata", "emptyMeasurements.json")),
			want:  testutil.GetConf(t, filepath.Join("testdata", "emptyMeasurements.yaml")),
		},
		"WithAgentInterval": {
			input:   testutil.GetJson(t, filepath.Join("testdata", "agentInterval.json")),
			wantErr: fmt.Errorf("measurement is required for efa receiver (%s)", tt.ID()),
		},
		"WithOverrideAgentInterval": {
			input:   testutil.GetJson(t, filepath.Join("testdata", "overrideInterval.json")),
			wantErr: fmt.Errorf("measurement is required for efa receiver (%s)", tt.ID()),
		},
	}
	factory := awsefareceiver.NewFactory()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.Equal(t, testCase.wantErr, err)
			if err == nil {
				require.NotNil(t, got)
				gotCfg, ok := got.(*awsefareceiver.Config)
				require.True(t, ok)
				wantCfg := factory.CreateDefaultConfig().(*awsefareceiver.Config)
				require.NoError(t, testCase.want.Unmarshal(wantCfg))
				compareConfigs(t, wantCfg, gotCfg)
			}
		})
	}
}

// compareConfigs compares two configs field by field to avoid issues with
// unexported fields (enabledSetByUser in MetricConfig).
func compareConfigs(t *testing.T, want, got *awsefareceiver.Config) {
	assert.Equal(t, want.CollectionInterval, got.CollectionInterval)
	assert.Equal(t, want.HostPath, got.HostPath)

	assert.Equal(t, want.Metrics.EfaImpairedRemoteConnEvents.Enabled, got.Metrics.EfaImpairedRemoteConnEvents.Enabled)
	assert.Equal(t, want.Metrics.EfaRdmaReadBytes.Enabled, got.Metrics.EfaRdmaReadBytes.Enabled)
	assert.Equal(t, want.Metrics.EfaRdmaReadRespBytes.Enabled, got.Metrics.EfaRdmaReadRespBytes.Enabled)
	assert.Equal(t, want.Metrics.EfaRdmaReadWrErr.Enabled, got.Metrics.EfaRdmaReadWrErr.Enabled)
	assert.Equal(t, want.Metrics.EfaRdmaReadWrs.Enabled, got.Metrics.EfaRdmaReadWrs.Enabled)
	assert.Equal(t, want.Metrics.EfaRdmaWriteBytes.Enabled, got.Metrics.EfaRdmaWriteBytes.Enabled)
	assert.Equal(t, want.Metrics.EfaRdmaWriteRecvBytes.Enabled, got.Metrics.EfaRdmaWriteRecvBytes.Enabled)
	assert.Equal(t, want.Metrics.EfaRdmaWriteWrErr.Enabled, got.Metrics.EfaRdmaWriteWrErr.Enabled)
	assert.Equal(t, want.Metrics.EfaRdmaWriteWrs.Enabled, got.Metrics.EfaRdmaWriteWrs.Enabled)
	assert.Equal(t, want.Metrics.EfaRecvBytes.Enabled, got.Metrics.EfaRecvBytes.Enabled)
	assert.Equal(t, want.Metrics.EfaRecvWrs.Enabled, got.Metrics.EfaRecvWrs.Enabled)
	assert.Equal(t, want.Metrics.EfaRetransBytes.Enabled, got.Metrics.EfaRetransBytes.Enabled)
	assert.Equal(t, want.Metrics.EfaRetransPkts.Enabled, got.Metrics.EfaRetransPkts.Enabled)
	assert.Equal(t, want.Metrics.EfaRetransTimeoutEvents.Enabled, got.Metrics.EfaRetransTimeoutEvents.Enabled)
	assert.Equal(t, want.Metrics.EfaRxBytes.Enabled, got.Metrics.EfaRxBytes.Enabled)
	assert.Equal(t, want.Metrics.EfaRxDropped.Enabled, got.Metrics.EfaRxDropped.Enabled)
	assert.Equal(t, want.Metrics.EfaRxPkts.Enabled, got.Metrics.EfaRxPkts.Enabled)
	assert.Equal(t, want.Metrics.EfaSendBytes.Enabled, got.Metrics.EfaSendBytes.Enabled)
	assert.Equal(t, want.Metrics.EfaSendWrs.Enabled, got.Metrics.EfaSendWrs.Enabled)
	assert.Equal(t, want.Metrics.EfaTxBytes.Enabled, got.Metrics.EfaTxBytes.Enabled)
	assert.Equal(t, want.Metrics.EfaTxPkts.Enabled, got.Metrics.EfaTxPkts.Enabled)
	assert.Equal(t, want.Metrics.EfaUnresponsiveRemoteEvents.Enabled, got.Metrics.EfaUnresponsiveRemoteEvents.Enabled)
}

func TestNewTranslator(t *testing.T) {
	translator := NewTranslator()
	assert.Equal(t, "awsefareceiver", translator.ID().String())

	customName := "custom_name"
	translator = NewTranslator(common.WithName(customName))
	assert.Equal(t, "awsefareceiver/"+customName, translator.ID().String())
}
