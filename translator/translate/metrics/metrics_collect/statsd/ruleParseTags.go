// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statsd

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
)

type ParseTags struct {
}

const SectionKey_ParseTags = "parse_data_dog_tags"

func (obj *ParseTags) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = translator.DefaultCase(SectionKey_ParseTags, true, input)
	return
}

func init() {
	obj := new(ParseTags)
	RegisterRule(SectionKey_ParseTags, obj)
}
