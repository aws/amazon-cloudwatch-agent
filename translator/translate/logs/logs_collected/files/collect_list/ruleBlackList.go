// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

const BlacklistSectionKey = "blacklist"

type BlackList struct {
}

func (f *BlackList) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase(BlacklistSectionKey, "", input)
	if returnVal == "" {
		return
	}
	returnKey = BlacklistSectionKey
	return
}

func init() {
	b := new(BlackList)
	r := []Rule{b}
	RegisterRule(BlacklistSectionKey, r)
}
