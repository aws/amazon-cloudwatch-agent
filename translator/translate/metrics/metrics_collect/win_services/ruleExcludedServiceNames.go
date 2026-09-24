// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package win_services

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type ExcludedServiceNames struct {
}

const SectionKey_ExcludedServiceNames = "excluded_service_names"

func (obj *ExcludedServiceNames) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	m, ok := input.(map[string]interface{})
	if !ok {
		return "", ""
	}
	if _, exists := m[SectionKey_ExcludedServiceNames]; !exists {
		return "", ""
	}
	return translator.DefaultStringArrayCase(SectionKey_ExcludedServiceNames, []string{}, input)
}

func init() {
	obj := new(ExcludedServiceNames)
	RegisterRule(SectionKey_ExcludedServiceNames, obj)
}
