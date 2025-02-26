// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list //nolint:revive

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const TrimTimestampSectionKey = "trim_timestamp"

type TrimTimestamp struct {
}

func (r *TrimTimestamp) ApplyRule(input interface{}) (string, interface{}) {
	_, returnVal := translator.DefaultCase(TrimTimestampSectionKey, "", input)
	if returnVal == "" {
		return "", ""
	}
	returnKey := TrimTimestampSectionKey
	var ok bool
	if returnVal, ok = returnVal.(bool); !ok {
		returnVal = false
	}
	return returnKey, returnVal
}

func init() {
	l := new(TrimTimestamp)
	r := []Rule{l}
	RegisterRule(TrimTimestampSectionKey, r)
}
