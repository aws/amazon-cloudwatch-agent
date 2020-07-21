// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package append_dimensions

type InstanceType struct {
}

const Reserved_Key_Instance_Type = "InstanceType"
const Reserved_Val_Instance_Type = "${aws:InstanceType}"

func (i *InstanceType) ApplyRule(input interface{}) (string, interface{}) {
	return CheckIfExactMatch(input, Reserved_Key_Instance_Type, Reserved_Val_Instance_Type, "ec2_metadata_tags", Reserved_Key_Instance_Type)
}

func init() {
	i := new(InstanceType)
	RegisterRule(Reserved_Key_Instance_Type, i)
}
