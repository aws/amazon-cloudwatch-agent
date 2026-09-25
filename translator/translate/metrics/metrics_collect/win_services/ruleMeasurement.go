// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package win_services

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type Measurement struct {
}

const SectionKey_Measurement = "measurement"

func (obj *Measurement) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m, ok := input.(map[string]interface{})
	if !ok {
		return "", ""
	}
	if _, exists := m[SectionKey_Measurement]; !exists {
		return "", ""
	}
	_, returnVal = translator.DefaultStringArrayCase(SectionKey_Measurement, []string{}, input)
	returnKey = "fieldpass"
	return
}

func init() {
	obj := new(Measurement)
	RegisterRule(SectionKey_Measurement, obj)
}
