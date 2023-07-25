// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

func TestCommonCredsConfig(t *testing.T) {
	r := new(CommonCreds)
	ctx := context.CurrentContext()
	ctx.SetMode(config.ModeOnPrem)
	input := map[string]interface{}{}
	r.ApplyRule(input)
	assert.Equal(t, "AmazonCloudWatchAgent", Global_Config.Credentials[Profile_Key], "profile checking")

	ctx.SetCredentials(map[string]string{commonconfig.CredentialProfile: "default"})
	r.ApplyRule(input)
	assert.Equal(t, "default", Global_Config.Credentials[Profile_Key], "profile checking")
	assert.Equal(t, util.DetectCredentialsPath(), Global_Config.Credentials[CredentialsFile_Key], "credentials path checking")

	ctx.SetCredentials(map[string]string{commonconfig.CredentialProfile: "default",
		commonconfig.CredentialFile: "/opt/test/credentials", "faked_key": "faked_value"})
	r.ApplyRule(input)
	assert.Equal(t, "default", Global_Config.Credentials[Profile_Key], "profile checking")
	assert.Equal(t, "/opt/test/credentials", Global_Config.Credentials[CredentialsFile_Key], "credentials path checking")
	assert.Equal(t, "faked_value", Global_Config.Credentials["faked_key"], "faked_key checking")

	ctx.SetCredentials(map[string]string{})
	ctx.SetMode(config.ModeEC2)
	r.ApplyRule(input)
	assert.Equal(t, nil, Global_Config.Credentials[Profile_Key], "profile checking")

	ctx.SetCredentials(map[string]string{commonconfig.CredentialProfile: "default"})
	r.ApplyRule(input)
	assert.Equal(t, "default", Global_Config.Credentials[Profile_Key], "profile checking")
}
