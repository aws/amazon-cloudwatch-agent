// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package sigv4auth

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	translateagent "github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
)

func TestTranslate(t *testing.T) {
	testCases := map[string]struct {
		service     string
		mode        string
		region      string
		roleARN     string
		profile     string
		credsFile   string
		wantID      component.ID
		wantProfile string
		wantFile    []string
		wantLocal   bool
		wantRole    sigv4authextension.AssumeRole
	}{
		"Default": {
			mode:   config.ModeEC2,
			region: "us-east-1",
			wantID: component.MustNewID("sigv4auth"),
		},
		"WithService": {
			service: "logs",
			mode:    config.ModeEC2,
			region:  "us-east-1",
			wantID:  component.MustNewIDWithName("sigv4auth", "logs"),
		},
		"WithProfile": {
			mode:        config.ModeEC2,
			region:      "us-east-1",
			profile:     "test-profile",
			wantID:      component.MustNewID("sigv4auth"),
			wantProfile: "test-profile",
		},
		"WithSharedCredentialsFile": {
			mode:      config.ModeEC2,
			region:    "us-east-1",
			credsFile: "/etc/aws/credentials",
			wantID:    component.MustNewID("sigv4auth"),
			wantFile:  []string{"/etc/aws/credentials"},
		},
		"WithProfileAndFile": {
			mode:        config.ModeEC2,
			region:      "us-east-1",
			profile:     "test-profile",
			credsFile:   "/etc/aws/credentials",
			wantID:      component.MustNewID("sigv4auth"),
			wantProfile: "test-profile",
			wantFile:    []string{"/etc/aws/credentials"},
		},
		"OnPremMode": {
			mode:      config.ModeOnPrem,
			region:    "us-east-1",
			wantID:    component.MustNewID("sigv4auth"),
			wantLocal: true,
		},
		"OnPremiseMode": {
			mode:      config.ModeOnPremise,
			region:    "us-east-1",
			wantID:    component.MustNewID("sigv4auth"),
			wantLocal: true,
		},
		"WithRoleARN": {
			mode:    config.ModeEC2,
			region:  "us-east-1",
			roleARN: "arn:aws:iam::123456789012:role/test-role",
			wantID:  component.MustNewID("sigv4auth"),
			wantRole: sigv4authextension.AssumeRole{
				ARN:       "arn:aws:iam::123456789012:role/test-role",
				STSRegion: "us-east-1",
			},
		},
		"OnPremWithProfileAndFileAndRole": {
			service:     "logs",
			mode:        config.ModeOnPrem,
			region:      "us-west-2",
			profile:     "AmazonCloudWatchAgent",
			credsFile:   "/opt/aws/amazon-cloudwatch-agent/etc/credentials",
			roleARN:     "arn:aws:iam::123456789012:role/agent",
			wantID:      component.MustNewIDWithName("sigv4auth", "logs"),
			wantProfile: "AmazonCloudWatchAgent",
			wantFile:    []string{"/opt/aws/amazon-cloudwatch-agent/etc/credentials"},
			wantLocal:   true,
			wantRole: sigv4authextension.AssumeRole{
				ARN:       "arn:aws:iam::123456789012:role/agent",
				STSRegion: "us-west-2",
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			resetGlobalState(t)
			translateagent.Global_Config.Region = testCase.region
			translateagent.Global_Config.Role_arn = testCase.roleARN
			if testCase.profile != "" {
				translateagent.Global_Config.Credentials[translateagent.Profile_Key] = testCase.profile
			}
			if testCase.credsFile != "" {
				translateagent.Global_Config.Credentials[translateagent.CredentialsFile_Key] = testCase.credsFile
			}
			context.CurrentContext().SetMode(testCase.mode)

			var tt common.ComponentTranslator
			if testCase.service != "" {
				tt = NewTranslatorWithService(testCase.service)
			} else {
				tt = NewTranslator()
			}
			assert.Equal(t, testCase.wantID, tt.ID())

			got, err := tt.Translate(confmap.NewFromStringMap(map[string]interface{}{}))
			require.NoError(t, err)
			gotCfg, ok := got.(*sigv4authextension.Config)
			require.True(t, ok)

			assert.Equal(t, testCase.region, gotCfg.Region)
			assert.Equal(t, testCase.service, gotCfg.Service)
			assert.Equal(t, testCase.wantProfile, gotCfg.Profile)
			assert.Equal(t, testCase.wantFile, gotCfg.SharedCredentialsFile)
			assert.Equal(t, testCase.wantLocal, gotCfg.LocalMode)
			assert.Equal(t, testCase.wantRole, gotCfg.AssumeRole)
		})
	}
}

// resetGlobalState clears agent state that the translator reads from. The package-level
// Global_Config is mutated by Translate, so each subtest must start from a clean slate.
func resetGlobalState(t *testing.T) {
	t.Helper()
	translateagent.Global_Config.Credentials = make(map[string]interface{})
	translateagent.Global_Config.Region = ""
	translateagent.Global_Config.Role_arn = ""
}
