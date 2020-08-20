// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package procstat

const (
	tagExcludeKey = "tagexclude"
)

var (
	tagExcludeValues = []string{"user", "result"}
)

type DropTags struct {
}

func (i *DropTags) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey, returnVal = tagExcludeKey, tagExcludeValues
	return
}

func init() {
	RegisterRule("dropTags", new(DropTags))
}
