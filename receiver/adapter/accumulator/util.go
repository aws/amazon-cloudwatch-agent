// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package accumulator

import (
	"runtime"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Otel Attributes = Telegraf Tags = CloudWatch Dimensions
func addTagsToAttributes(attributes pcommon.Map, tags map[string]string) {
	for tag, value := range tags {
		attributes.PutStr(tag, value)
	}
}

// Adapted from http://github.com/aws/amazon-cloudwatch-agent/blob/40bb174c0e2309da6bd2c6e1a36c501324b2d6b0/plugins/outputs/cloudwatch/cloudwatch.go#L385-L385
func getMetricName(measurement string, fieldKey string) string {
	// Statsd sets field name as default when the field is empty
	// https://github.com/aws/amazon-cloudwatch-agent/blob/6b3384ee44dcc07c1359b075eb9ea8e638126bc8/plugins/inputs/statsd/statsd.go#L492-L494
	if fieldKey == "value" {
		return measurement
	}

	separator := "_"

	if runtime.GOOS == "windows" {
		separator = " "
	}

	return strings.Join([]string{measurement, fieldKey}, separator)
}
