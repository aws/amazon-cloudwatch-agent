// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package accumulator

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Otel Attributes = Telegraf Tags = CloudWatch Dimensions
func addTagsToAttributes(attributes pcommon.Map, tags map[string]string) {
	for tag, value := range tags {
		attributes.PutString(tag, value)
	}
}

// Add measurement as a global attribute
func addMeasurementNameAsAttribute(attributes pcommon.Map, measurement string) {
	attributes.PutString(measurementAttribute, measurement)
}

// Adapted from http://github.com/aws/amazon-cloudwatch-agent/blob/40bb174c0e2309da6bd2c6e1a36c501324b2d6b0/plugins/outputs/cloudwatch/cloudwatch.go#L385-L385
func getMetricName(measurement string, fieldKey string) string {
	separator := "_"
	return strings.Join([]string{measurement, fieldKey}, separator)
}
