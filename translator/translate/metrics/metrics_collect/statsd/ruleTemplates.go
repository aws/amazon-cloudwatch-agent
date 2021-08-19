// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statsd

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type Templates struct {
}

const SectionKey_Templates = "templates"

func (obj *Templates) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, inputTemplates := translator.DefaultStringArrayCase(SectionKey_Templates, []string{}, input)
	returnVal = inputTemplates.([]string)
	return
}

func init() {
	obj := new(Templates)
	RegisterRule(SectionKey_Templates, obj)
}
