// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package cloudauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	extcloudauth "github.com/aws/amazon-cloudwatch-agent/extension/cloudauth"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

func TestTranslator_ID(t *testing.T) {
	tr := NewTranslator()
	assert.Equal(t, "cloudauth", tr.ID().String())
}

func TestTranslator_Translate(t *testing.T) {
	tests := map[string]struct {
		globalRegion  string
		globalRoleARN string
		input         map[string]interface{}
		wantRegion    string
		wantRoleARN   string
		wantTokenFile string
	}{
		"GlobalConfig": {
			globalRegion:  "us-west-2",
			globalRoleARN: "arn:aws:iam::123456789012:role/GlobalRole",
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"credentials": map[string]interface{}{
						"oidc_auth": map[string]interface{}{},
					},
				},
			},
			wantRegion:  "us-west-2",
			wantRoleARN: "arn:aws:iam::123456789012:role/GlobalRole",
		},
		"MetricsRoleOverride": {
			globalRegion:  "us-east-1",
			globalRoleARN: "arn:aws:iam::123456789012:role/GlobalRole",
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"credentials": map[string]interface{}{
						"oidc_auth": map[string]interface{}{},
					},
				},
				"metrics": map[string]interface{}{
					"credentials": map[string]interface{}{
						"role_arn": "arn:aws:iam::123456789012:role/MetricsRole",
					},
				},
			},
			wantRegion:  "us-east-1",
			wantRoleARN: "arn:aws:iam::123456789012:role/MetricsRole",
		},
		"LogsRoleOverride": {
			globalRegion:  "eu-west-1",
			globalRoleARN: "arn:aws:iam::123456789012:role/GlobalRole",
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"credentials": map[string]interface{}{
						"oidc_auth": map[string]interface{}{},
					},
				},
				"logs": map[string]interface{}{
					"credentials": map[string]interface{}{
						"role_arn": "arn:aws:iam::123456789012:role/LogsRole",
					},
				},
			},
			wantRegion:  "eu-west-1",
			wantRoleARN: "arn:aws:iam::123456789012:role/LogsRole",
		},
		"WithTokenFile": {
			globalRegion:  "us-east-1",
			globalRoleARN: "arn:aws:iam::123456789012:role/TestRole",
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"credentials": map[string]interface{}{
						"oidc_auth": map[string]interface{}{
							"token_file": "/var/run/oidc/token",
						},
					},
				},
			},
			wantRegion:    "us-east-1",
			wantRoleARN:   "arn:aws:iam::123456789012:role/TestRole",
			wantTokenFile: "/var/run/oidc/token",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			agent.Global_Config.Region = tc.globalRegion
			agent.Global_Config.Role_arn = tc.globalRoleARN

			conf := confmap.NewFromStringMap(tc.input)
			tr := NewTranslator()
			got, err := tr.Translate(conf)
			require.NoError(t, err)

			cfg, ok := got.(*extcloudauth.Config)
			require.True(t, ok)
			assert.Equal(t, tc.wantRegion, cfg.Region)
			assert.Equal(t, tc.wantRoleARN, cfg.RoleARN)
			assert.Equal(t, tc.wantTokenFile, cfg.TokenFile)
		})
	}
}

func TestIsSet(t *testing.T) {
	tests := map[string]struct {
		input map[string]interface{}
		want  bool
	}{
		"Set": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"credentials": map[string]interface{}{
						"oidc_auth": map[string]interface{}{},
					},
				},
			},
			want: true,
		},
		"NotSet": {
			input: map[string]interface{}{
				"agent": map[string]interface{}{
					"credentials": map[string]interface{}{
						"role_arn": "some-arn",
					},
				},
			},
			want: false,
		},
		"Empty": {
			input: map[string]interface{}{},
			want:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			conf := confmap.NewFromStringMap(tc.input)
			assert.Equal(t, tc.want, IsSet(conf))
		})
	}
}
