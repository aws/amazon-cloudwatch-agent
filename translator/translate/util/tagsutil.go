// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

const (
	High_Resolution_Tag_Key      = "aws:StorageResolution"
	Aggregation_Interval_Tag_Key = "aws:AggregationInterval"
)

var Reserved_Tag_Keys = []string{High_Resolution_Tag_Key, Aggregation_Interval_Tag_Key}

func AddHighResolutionTag(tags interface{}) {
	tagMap := tags.(map[string]interface{})
	tagMap[High_Resolution_Tag_Key] = "true"
}

// Filter out reserved tag keys
func Cleanup(input interface{}) {
	inputmap := input.(map[string]interface{})
	for _, reserved_key := range Reserved_Tag_Keys {
		delete(inputmap, reserved_key)
	}
}
