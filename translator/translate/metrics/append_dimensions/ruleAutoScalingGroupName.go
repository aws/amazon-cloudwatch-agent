// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package append_dimensions

type AutoScalingGroupName struct {
}

const Reserved_Key_ASG = "AutoScalingGroupName"
const Reserved_Val_ASG = "${aws:AutoScalingGroupName}"

func (a *AutoScalingGroupName) ApplyRule(input interface{}) (string, interface{}) {
	return CheckIfExactMatch(input, Reserved_Key_ASG, Reserved_Val_ASG, "ec2_instance_tag_keys", "aws:autoscaling:groupName")
}

func init() {
	a := new(AutoScalingGroupName)
	RegisterRule(Reserved_Key_ASG, a)
}
