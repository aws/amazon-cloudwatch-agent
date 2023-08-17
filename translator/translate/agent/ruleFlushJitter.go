// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type FlushJitter struct {
}

func (f *FlushJitter) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("flush_jitter", "0s", input)
	return
}

func init() {
	f := new(FlushJitter)
	RegisterRule("flush_jitter", f)
}
