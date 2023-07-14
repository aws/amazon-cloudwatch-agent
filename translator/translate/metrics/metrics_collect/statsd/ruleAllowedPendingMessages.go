// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statsd

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type AllowedPendingMessages struct {
}

const SectionKey_AllowedPendingMessages = "allowed_pending_messages"

func (obj *AllowedPendingMessages) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKey_AllowedPendingMessages, "", input)
	if returnVal != "" {
		// By default json unmarshal will store number as float64
		return returnKey, int(returnVal.(float64))
	}
	return "", nil
}

func init() {
	obj := new(AllowedPendingMessages)
	RegisterRule(SectionKey_AllowedPendingMessages, obj)
}
