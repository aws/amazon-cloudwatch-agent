// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/agent"

type Region struct {
	returnTargetKey string
}

// Grant the global creds(if exist)
func (r *Region) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey = r.returnTargetKey
	returnVal = map[string]interface{}{"region": agent.Global_Config.Region}
	return
}

func GetRegionRule(returnTargetKey string) *Region {
	r := new(Region)
	r.returnTargetKey = returnTargetKey
	return r
}
