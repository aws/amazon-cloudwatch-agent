// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package otlp

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected"
)

type Rule translator.Rule

type Otlp struct {
}

const SectionKey = "otlp"

func GetCurPath() string {
	curPath := parent.GetCurPath() + SectionKey + "/"
	return curPath
}

func (o *Otlp) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	im := input.(map[string]interface{})
	result := map[string]map[string]interface{}{}
	inputs := map[string]interface{}{}
	processors := map[string]interface{}{}

	// Check if this plugin exists in the input instance
	if _, ok := im[SectionKey]; !ok {
		returnKey = ""
		returnVal = ""
	} else {
		// OTLP configuration is handled by the OTEL pipeline translator
		// This rule just validates the configuration exists
		result["inputs"] = inputs
		result["processors"] = processors
		returnKey = SectionKey
		returnVal = result
	}
	return
}

func init() {
	o := new(Otlp)
	parent.RegisterLinuxRule(SectionKey, o)
	parent.RegisterDarwinRule(SectionKey, o)
	parent.RegisterWindowsRule(SectionKey, o)
}
