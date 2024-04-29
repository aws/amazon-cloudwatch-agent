// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import "github.com/aws/amazon-cloudwatch-agent/internal/util/collections"

const (
	High_Resolution_Tag_Key      = "aws:StorageResolution"
	Aggregation_Interval_Tag_Key = "aws:AggregationInterval"
	VolumeIdTagKey               = "EBSVolumeId"
)

var ReservedTagKeySet = collections.NewSet(High_Resolution_Tag_Key, Aggregation_Interval_Tag_Key, VolumeIdTagKey)

func AddHighResolutionTag(tags interface{}) {
	tagMap := tags.(map[string]interface{})
	tagMap[High_Resolution_Tag_Key] = "true"
}

// FilterReservedKeys out reserved tag keys
func FilterReservedKeys(input any) any {
	result := map[string]any{}
	for k, v := range input.(map[string]interface{}) {
		if !ReservedTagKeySet.Contains(k) {
			result[k] = v
		}
	}
	return result
}
