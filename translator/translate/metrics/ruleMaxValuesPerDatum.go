// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package metrics

import "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/agent"

type MaxValuesPerDatum struct {
}

func (obj *MaxValuesPerDatum) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	if agent.Global_Config.Internal {
		res := map[string]interface{}{"max_values_per_datum": 5000}
		returnKey = "outputs"
		returnVal = res
	}
	return
}

func init() {
	obj := new(MaxValuesPerDatum)
	RegisterRule("max_values_per_datum", obj)
}
