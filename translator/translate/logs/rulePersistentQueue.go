// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package logs

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type PersistentQueue struct {
}

func (p *PersistentQueue) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	res := map[string]interface{}{}
	key, val := translator.DefaultCase("persistentQueue", false, input)
	res[key] = val
	if val != "" {
		returnKey = Output_Cloudwatch_Logs
		returnVal = res
	}
	return
}
func init() {
	p := new(PersistentQueue)
	RegisterRule("persistentQueue", p)
}
