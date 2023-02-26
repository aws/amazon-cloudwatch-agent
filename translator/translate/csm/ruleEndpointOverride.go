// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package csm

import (
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent/translator"

	"github.com/aws/amazon-cloudwatch-agent/internal/csm"
)

type EndpointOverride struct{}

func (*EndpointOverride) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, val := translator.DefaultCase(csm.EndpointOverrideKey, "", input)

	v, ok := val.(string)
	if !ok {
		translator.AddErrorMessages(
			ConfOutputPluginKey+"/"+csm.EndpointOverrideKey,
			fmt.Sprintf("value (%v) is not a string, but %T", val, val),
		)
		return
	}
	if len(v) == 0 {
		return
	}

	res := map[string]interface{}{}
	res[csm.EndpointOverrideKey] = val

	returnKey = ConfOutputPluginKey
	returnVal = res

	return
}

func init() {
	o := new(EndpointOverride)
	RegisterRule(csm.EndpointOverrideKey, o)
}
