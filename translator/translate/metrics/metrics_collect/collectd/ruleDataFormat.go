// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package collected

type ParseTags struct {
}

const SectionKey_DataFormat = "data_format"

func (obj *ParseTags) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	return SectionKey_DataFormat, "collectd"
}

func init() {
	obj := new(ParseTags)
	RegisterRule(SectionKey_DataFormat, obj)
}
