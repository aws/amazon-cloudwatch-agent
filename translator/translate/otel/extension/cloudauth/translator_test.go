// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/cloudauthextension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestTranslator_Translate(t *testing.T) {
	tests := map[string]struct {
		input         map[string]interface{}
		wantTokenFile string
		wantAudience  string
	}{
		"EmptyOIDCAuth": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"credentials": map[string]interface{}{
						"oidc_auth": map[string]interface{}{},
					},
				},
			},
		},
		"WithTokenFile": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"credentials": map[string]interface{}{
						"oidc_auth": map[string]interface{}{
							"token_file": "/path/to/token",
							"audience":   "sts.amazonaws.com",
						},
					},
				},
			},
			wantTokenFile: "/path/to/token",
			wantAudience:  "sts.amazonaws.com",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tr := NewTranslator()
			assert.Equal(t, "cloudauth", tr.ID().String())

			cfg, err := tr.Translate(confmap.NewFromStringMap(tc.input))
			require.NoError(t, err)
			c := cfg.(*cloudauthextension.Config)
			assert.Equal(t, tc.wantTokenFile, c.TokenFile)
			assert.Equal(t, tc.wantAudience, c.Audience)
		})
	}
}

func TestIsSet(t *testing.T) {
	set := map[string]interface{}{
		"agent": map[string]interface{}{
			"credentials": map[string]interface{}{
				"oidc_auth": map[string]interface{}{},
			},
		},
	}
	assert.True(t, IsSet(confmap.NewFromStringMap(set)))
	assert.False(t, IsSet(confmap.NewFromStringMap(map[string]interface{}{})))
}
