// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const BackpressureDropSectionKey = "backpressure_drop"

type BackpressureDrop struct {
}

func (hr *BackpressureDrop) ApplyRule(input any) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase(BackpressureDropSectionKey, "", input)
	if returnVal == "" {
		// check for env var as fallback
		returnVal = envconfig.IsBackpressureDropEnabled()
		if !returnVal.(bool) {
			return
		}
	}
	returnKey = BackpressureDropSectionKey
	var ok bool
	if returnVal, ok = returnVal.(bool); !ok {
		returnVal = false
	}
	return
}

func init() {
	l := new(BackpressureDrop)
	r := []Rule{l}
	RegisterRule(BackpressureDropSectionKey, r)
}
