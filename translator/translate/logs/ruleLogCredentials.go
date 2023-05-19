// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/agent"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/util"
)

type LogCreds struct {
}

const (
	Role_Arn_Key          = "role_arn"
	CredentialsSectionKey = "credentials"
)

var credsTargetList = []string{Role_Arn_Key}

func (c *LogCreds) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	result := map[string]interface{}{}

	if agent.Global_Config.Role_arn != "" {
		result[Role_Arn_Key] = agent.Global_Config.Role_arn
	}

	// Read fromm Json first.
	if val, ok := input.(map[string]interface{})[CredentialsSectionKey]; ok {
		util.SetWithSameKeyIfFound(val, credsTargetList, result)
	}

	returnKey = Output_Cloudwatch_Logs
	returnVal = result
	return
}

func init() {
	c := new(LogCreds)
	RegisterRule(CredentialsSectionKey, c)
}
