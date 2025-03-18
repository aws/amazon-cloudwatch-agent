// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//nolint:revive // bypass lint check on new files
package collect_list

import (
	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const BackpressureDropSectionKey = "backpressure_drop"

type BackpressureDrop struct {
}

func (hr *BackpressureDrop) ApplyRule(input any) (string, interface{}) {
	_, returnVal := translator.DefaultCase(BackpressureDropSectionKey, "", input)
	if returnVal == "" {
		// check for env var as fallback
		returnVal = envconfig.IsBackpressureDropEnabled()
		if !returnVal.(bool) {
			return "", nil
		}
	}
	returnKey := BackpressureDropSectionKey
	var ok bool
	if returnVal, ok = returnVal.(bool); !ok {
		returnVal = false
	}
	return returnKey, returnVal
}

func init() {
	l := new(BackpressureDrop)
	r := []Rule{l}
	RegisterRule(BackpressureDropSectionKey, r)
}
