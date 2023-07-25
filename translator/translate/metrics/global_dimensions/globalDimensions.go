// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package global_dimensions

import (
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/metrics"
)

type globalDimensions struct {
}

const SectionKey = "global_dimensions"

func (ad *globalDimensions) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	dimensions := map[string]string{}

	returnKey = ""
	returnVal = ""

	if globalDimensionsMap, ok := im[SectionKey].(map[string]interface{}); ok {
		for key, val := range globalDimensionsMap {
			stringValue, valueIsString := val.(string)

			if key != "" && key != "host" && valueIsString && val != "" {
				dimensions[key] = stringValue
			}
		}

		if len(dimensions) > 0 {
			returnKey = "outputs"
			returnVal = map[string]interface{}{
				SectionKey: dimensions,
			}
		}
	}
	return
}

func init() {
	gd := new(globalDimensions)
	parent.RegisterRule(SectionKey, gd)
}
