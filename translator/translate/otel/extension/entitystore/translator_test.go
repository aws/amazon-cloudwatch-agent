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
	context.CurrentContext().SetMode(config.ModeEC2)
	translateagent.Global_Config.Credentials = make(map[string]interface{})
	testCases := map[string]struct {
		input          map[string]interface{}
		file_exists    bool
		profile_exists bool
		want           *entitystore.Config
	}{
		"OnlyProfile": {
			input:          map[string]interface{}{},
			profile_exists: true,
			want: &entitystore.Config{
				Mode:    config.ModeEC2,
				Profile: "test_profile",
			},
		},
		"OnlyFile": {
			input:       map[string]interface{}{},
			file_exists: true,
			want: &entitystore.Config{
				Mode:     config.ModeEC2,
				Filename: "test_file",
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			translateagent.Global_Config.Credentials[translateagent.Profile_Key] = ""
			translateagent.Global_Config.Credentials[translateagent.CredentialsSectionKey] = ""
			if testCase.file_exists {
				translateagent.Global_Config.Credentials[translateagent.CredentialsFile_Key] = "test_file"
			}
			if testCase.profile_exists {
				translateagent.Global_Config.Credentials[translateagent.Profile_Key] = "test_profile"
			}
			tt := NewTranslator().(*translator)
			assert.Equal(t, "entitystore", tt.ID().String())
			conf := confmap.NewFromStringMap(testCase.input)
			got, err := tt.Translate(conf)
			assert.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}
