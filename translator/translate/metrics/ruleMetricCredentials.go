// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/agent"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/util"
)

type MetricsCreds struct {
}

const (
	Role_Arn_Key          = "role_arn"
	CredentialsSectionKey = "credentials"
)

var credsTargetList = []string{Role_Arn_Key}

func (c *MetricsCreds) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	result := map[string]interface{}{}

	if agent.Global_Config.Role_arn != "" {
		result[Role_Arn_Key] = agent.Global_Config.Role_arn
	}

	// Read fromm Json first.
	if val, ok := input.(map[string]interface{})[CredentialsSectionKey]; ok {
		util.SetWithSameKeyIfFound(val, credsTargetList, result)
	}

	returnKey = OutputsKey
	returnVal = result
	return
}

func init() {
	c := new(MetricsCreds)
	RegisterRule(CredentialsSectionKey, c)
}
