// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/util"
)

type GlobalCreds struct {
}

const (
	Role_Arn_Key          = "role_arn"
	CredentialsSectionKey = "credentials"
)

var credsTargetList = []string{Role_Arn_Key}

func (c *GlobalCreds) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	result := map[string]interface{}{}

	// Read from Json first.
	if val, ok := input.(map[string]interface{})[CredentialsSectionKey]; ok {
		util.SetWithSameKeyIfFound(val, credsTargetList, result)
	}

	if role_arn, exist := result[Role_Arn_Key]; exist {
		Global_Config.Role_arn = role_arn.(string)
	}

	return
}

func init() {
	c := new(GlobalCreds)
	RegisterRule(CredentialsSectionKey, c)
}
