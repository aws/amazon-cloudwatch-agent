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
	_, val := translator.DefaultCase(TrimTimestampSectionKey, "", input)
	if val == "" {
		return "", ""
	}

	boolVal, ok := val.(bool)
	if !ok {
		return TrimTimestampSectionKey, false
	}

	return TrimTimestampSectionKey, boolVal
}

func init() {
	l := new(TrimTimestamp)
	r := []Rule{l}
	RegisterRule(TrimTimestampSectionKey, r)
}
