// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package entitystore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	translateagent "github.com/aws/amazon-cloudwatch-agent/translator/translate/agent"
)

func TestTranslate(t *testing.T) {
	translateagent.Global_Config.Credentials = make(map[string]interface{})
	translateagent.Global_Config.Region = "us-east-1"
	testCases := map[string]struct {
		input          map[string]interface{}
		inputMode      string
		inputK8sMode   string
		file_exists    bool
		profile_exists bool
		want           *entitystore.Config
	}{
		"OnlyProfile": {
			input:          map[string]interface{}{},
			inputMode:      config.ModeEC2,
			inputK8sMode:   config.ModeEKS,
			profile_exists: true,
			want: &entitystore.Config{
				Mode:           config.ModeEC2,
				KubernetesMode: config.ModeEKS,
				Region:         "us-east-1",
				Profile:        "test_profile",
			},
		},
		"OnlyProfileWithK8sOnPrem": {
			input:          map[string]interface{}{},
			inputMode:      config.ModeEC2,
			inputK8sMode:   config.ModeK8sOnPrem,
			profile_exists: true,
			want: &entitystore.Config{
				Mode:           config.ModeEC2,
				KubernetesMode: config.ModeK8sOnPrem,
				Region:         "us-east-1",
				Profile:        "test_profile",
			},
		},
		"OnlyFile": {
			input:        map[string]interface{}{},
			inputMode:    config.ModeEC2,
			inputK8sMode: config.ModeK8sEC2,
			file_exists:  true,
			want: &entitystore.Config{
				Mode:           config.ModeEC2,
				KubernetesMode: config.ModeK8sEC2,
				Region:         "us-east-1",
				Filename:       "test_file",
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			translateagent.Global_Config.Credentials[translateagent.CredentialsSectionKey] = ""
			if testCase.file_exists {
				translateagent.Global_Config.Credentials[translateagent.CredentialsFile_Key] = "test_file"
				translateagent.Global_Config.Credentials[translateagent.Profile_Key] = ""
			}
			if testCase.profile_exists {
				translateagent.Global_Config.Credentials[translateagent.Profile_Key] = "test_profile"
				translateagent.Global_Config.Credentials[translateagent.CredentialsFile_Key] = ""
			}
			tt := NewTranslator().(*translator)
			assert.Equal(t, "entitystore", tt.ID().String())
			conf := confmap.NewFromStringMap(testCase.input)
			context.CurrentContext().SetMode(testCase.inputMode)
			context.CurrentContext().SetKubernetesMode(testCase.inputK8sMode)
			got, err := tt.Translate(conf)
			assert.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}
