// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package emf

type NameOverride struct {
}

const SectionKeyNameOverride = "name_override"

func (obj *NameOverride) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	return SectionKeyNameOverride, "emf"
}

func init() {
	obj := new(NameOverride)
	RegisterRule(SectionKeyNameOverride, obj)
}
