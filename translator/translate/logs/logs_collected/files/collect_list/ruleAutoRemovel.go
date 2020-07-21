// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const AutoRemovalSectionKey = "auto_removal"

type AutoRemoval struct {
}

func (r *AutoRemoval) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase(AutoRemovalSectionKey, "", input)
	if returnVal == "" {
		return
	}
	returnKey = AutoRemovalSectionKey
	var ok bool
	if returnVal, ok = returnVal.(bool); !ok {
		returnVal = false
	}
	return
}

func init() {
	l := new(AutoRemoval)
	r := []Rule{l}
	RegisterRule(AutoRemovalSectionKey, r)
}
