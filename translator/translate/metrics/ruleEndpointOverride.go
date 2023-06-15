// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type EndpointOverride struct {
}

func (r *EndpointOverride) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	res := map[string]interface{}{}
	key, val := translator.DefaultCase("endpoint_override", "", input)
	res[key] = val
	if val != "" {
		returnKey = "outputs"
		returnVal = res
	}
	return
}

func init() {
	r := new(EndpointOverride)
	RegisterRule("endpoint_override", r)
}
