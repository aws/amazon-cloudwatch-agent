// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package awsproxy

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslate(t *testing.T) {
	tt := NewTranslator()
	conf := confmap.NewFromStringMap(map[string]any{"traces": map[string]any{}})
	got, err := tt.Translate(conf)
	if err == nil {
		require.NotNil(t, got)
		gotCfg, ok := got.(*awsproxy.Config)
		require.True(t, ok)
		wantCfg := awsproxy.NewFactory().CreateDefaultConfig().(*awsproxy.Config)
		wantCfg.ProxyConfig.IMDSRetries = 1
		// Upstream defaults to localhost but we want to stick with 0.0.0.0 as the default for AppSignals
		wantCfg.ProxyConfig.Endpoint = "0.0.0.0:2000"
		assert.Equal(t, wantCfg, gotCfg)
	}
}
