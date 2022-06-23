// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emf

import (
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator"
	"github.com/aws/private-amazon-cloudwatch-agent-staging/translator/context"
	parent "github.com/aws/private-amazon-cloudwatch-agent-staging/translator/translate/logs/metrics_collected"
)

//
// Need to import new rule package in src/translator/totomlconfig/toTomlConfig.go
//

//
//   "structuredlog" : {
//       "service_address": "udp://127.0.0.1:25888"
//   }
//
const SectionKeyStructuredLog = "structuredlog"

type StructuredLog struct {
}

func (obj *StructuredLog) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m := input.(map[string]interface{})
	resArray := []interface{}{}
	result := map[string]interface{}{}
	//Check if this plugin exist in the input instance
	//If not, not process
	if _, ok := m[SectionKeyStructuredLog]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		//If exists, process it
		//Check if there are some config entry with rules applied
		if sectionMap, ok := m[SectionKeyStructuredLog].(map[string]interface{}); ok && len(sectionMap) == 0 {
			// not configured
			defaultEndpointSuffix := "://127.0.0.1:25888"
			if context.CurrentContext().RunInContainer() {
				defaultEndpointSuffix = "://:25888"
			}
			resArray = []interface{}{
				map[string]interface{}{
					"service_address": "udp" + defaultEndpointSuffix,
					"data_format":     "emf",
					"name_override":   "emf",
				},
				map[string]interface{}{
					"service_address": "tcp" + defaultEndpointSuffix,
					"data_format":     "emf",
					"name_override":   "emf",
				},
			}
		} else {
			result = translator.ProcessRuleToApply(m[SectionKeyStructuredLog], ChildRule, result)
			resArray = append(resArray, result)
		}
		returnKey = "socket_listener"
		returnVal = resArray
	}
	return
}

func init() {
	obj := new(StructuredLog)
	parent.RegisterLinuxRule(SectionKeyStructuredLog, obj)
	parent.RegisterDarwinRule(SectionKeyStructuredLog, obj)
	parent.RegisterWindowsRule(SectionKeyStructuredLog, obj)
}
