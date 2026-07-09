// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator/config"
	"github.com/aws/amazon-cloudwatch-agent/translator/context"
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

type CommonCreds struct {
}

const (
	Profile_Key                 = "profile"
	CredentialsFile_Key         = "shared_credential_file"
	CommonCredentialsSectionKey = "commoncredentials"
)

// Here we simply record the credentials map into the agent section(global).
// This credential map will be provided to the corresponding input and output plugins
// This should be applied before interpreting other component.
func (c *CommonCreds) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	result := map[string]interface{}{}

	// Read from common-toml
	ctx := context.CurrentContext()
	credsMapFromCtx := util.GetCredentials(ctx.Mode(), ctx.Credentials())

	if len(credsMapFromCtx) > 0 {
		keyMapping := map[string]string{
			commonconfig.CredentialProfile: Profile_Key,
			commonconfig.CredentialFile:    CredentialsFile_Key,
		}
		for k, v := range credsMapFromCtx {
			if mappedk, ok := keyMapping[k]; ok {
				result[mappedk] = v
			} else {
				result[k] = v
			}
		}
	}

	if _, ok := result[Profile_Key]; ok {
		if _, ok := result[CredentialsFile_Key]; !ok {
			// Use default credential path at present
			result[commonconfig.CredentialFile] = util.DetectCredentialsPath()
		}
	}

	Global_Config.Credentials = result

	return
}

// HasSharedCredentials reports whether common-config.toml supplied a profile or credentials file.
func HasSharedCredentials() bool {
	_, hasProfile := Global_Config.Credentials[Profile_Key]
	_, hasFile := Global_Config.Credentials[CredentialsFile_Key]
	return hasProfile || hasFile
}

// IsAzureWebIdentity reports whether the oidctoken IMDS web-identity chain applies: plain Azure VM (not AKS, which uses the chart's projected SA token) with no common-config credentials.
func IsAzureWebIdentity() bool {
	ctx := context.CurrentContext()
	return ctx.Mode() == config.ModeAzureVM && ctx.KubernetesMode() != config.ModeAKS && !HasSharedCredentials()
}

func init() {
	c := new(CommonCreds)
	RegisterRule(CommonCredentialsSectionKey, c)
}
