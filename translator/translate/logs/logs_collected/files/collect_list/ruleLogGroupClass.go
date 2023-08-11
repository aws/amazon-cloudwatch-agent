// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

const LogGroupClassSectionKey = "log_group_class"

type LogGroupClass struct {
}

func (f *LogGroupClass) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultLogGroupClassCase(LogGroupClassSectionKey, "standard", input)
	returnKey = LogGroupClassSectionKey
	return
}

func init() {
	l := new(LogGroupClass)
	r := []Rule{l}
	RegisterRule(LogGroupClassSectionKey, r)
}
