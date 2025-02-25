// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const TrimTimestampSectionKey = "trim_timestamp"

type TrimTimestamp struct {
}

func (r *TrimTimestamp) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase(TrimTimestampSectionKey, "", input)
	if returnVal == "" {
		return
	}
	returnKey = TrimTimestampSectionKey
	var ok bool
	if returnVal, ok = returnVal.(bool); !ok {
		returnVal = false
	}
	return
}

func init() {
	l := new(TrimTimestamp)
	r := []Rule{l}
	RegisterRule(TrimTimestampSectionKey, r)
}