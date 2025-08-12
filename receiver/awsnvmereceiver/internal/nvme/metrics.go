// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import "go.opentelemetry.io/collector/pdata/pcommon"

// NVMeMetrics is a common interface implemented by all NVMe metric structs.
type NVMeMetrics interface {
	IsNVMeMetrics()
}

// RecordMetricFunc is a function type to record a metric data point.
type RecordMetricFunc func(recordFn func(pcommon.Timestamp, int64), ts pcommon.Timestamp, val uint64)
