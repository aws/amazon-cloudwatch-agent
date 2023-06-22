// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type FlushInterval struct {
}

func (f *FlushInterval) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase("flush_interval", "1s", input)
	return
}

func init() {
	f := new(FlushInterval)
	RegisterRule("flush_interval", f)
}
