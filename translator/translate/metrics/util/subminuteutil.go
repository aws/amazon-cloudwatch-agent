// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package util

import "time"

const Metric_High_Resolution_Threhold = 60 * time.Second

func IsHighResolution(intervalVal string) bool {
	if actualInterval, err := time.ParseDuration(intervalVal); err == nil {
		if actualInterval < Metric_High_Resolution_Threhold {
			return true
		}
	}
	return false
}
