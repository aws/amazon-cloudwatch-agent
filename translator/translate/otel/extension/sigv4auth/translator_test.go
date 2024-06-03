// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package sigv4auth

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslate(t *testing.T) {
	tt := NewTranslator()
	conf := confmap.NewFromStringMap(map[string]interface{}{})
	got, err := tt.Translate(conf)
	if err == nil {
		require.NotNil(t, got)
		gotCfg, ok := got.(*sigv4authextension.Config)
		require.True(t, ok)
		wantCfg := sigv4authextension.NewFactory().CreateDefaultConfig()
		assert.Equal(t, wantCfg, gotCfg)
	}
}
