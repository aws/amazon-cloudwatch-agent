// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type EndpointOverride struct {
}

func (r *EndpointOverride) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	res := map[string]interface{}{}
	key, val := translator.DefaultCase("endpoint_override", "", input)
	res[key] = val
	if val != "" {
		returnKey = Output_Cloudwatch_Logs
		returnVal = res
	}
	return
}
func init() {
	r := new(EndpointOverride)
	RegisterRule("endpoint_override", r)
}
