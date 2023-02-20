// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
)

type Namespace struct {
}

func (n *Namespace) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	res := map[string]interface{}{}
	key, val := translator.DefaultCase("namespace", "CWAgent", input)
	res[key] = val
	returnKey = "outputs"
	returnVal = res
	return
}

func init() {
	n := new(Namespace)
	RegisterRule("namespace", n)
}
