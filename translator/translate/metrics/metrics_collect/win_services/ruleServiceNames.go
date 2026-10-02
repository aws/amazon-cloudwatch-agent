// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package win_services

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type ServiceNames struct {
}

const SectionKey_ServiceNames = "service_names"

func (obj *ServiceNames) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m, ok := input.(map[string]interface{})
	if !ok {
		return "", ""
	}
	if _, exists := m[SectionKey_ServiceNames]; !exists {
		return "", ""
	}
	return translator.DefaultStringArrayCase(SectionKey_ServiceNames, []string{}, input)
}

func init() {
	obj := new(ServiceNames)
	RegisterRule(SectionKey_ServiceNames, obj)
}
