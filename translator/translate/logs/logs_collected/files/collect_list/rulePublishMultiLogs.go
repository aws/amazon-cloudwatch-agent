// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collect_list

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

const PublishMultiLogsSectionKey = "publish_multi_logs"

type PublishMultiLogs struct {
}

func (l *PublishMultiLogs) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	_, returnVal = translator.DefaultCase(PublishMultiLogsSectionKey, "", input)
	if returnVal == "" {
		return
	}
	returnKey = PublishMultiLogsSectionKey
	var ok bool
	if returnVal, ok = returnVal.(bool); !ok {
		returnVal = false
	}
	return
}

func init() {
	l := new(PublishMultiLogs)
	r := []Rule{l}
	RegisterRule(PublishMultiLogsSectionKey, r)
}
