// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package nvme

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/aws/amazon-cloudwatch-agent/receiver/awsnvmereceiver/internal/metadata"
)

// RecordMetricFunc is a function type to record a metric data point.
type RecordMetricFunc func(recordFn func(pcommon.Timestamp, int64), ts pcommon.Timestamp, val uint64)

// Metrics is a common interface implemented by all NVMe metric structs.
type Metrics interface {
	IsNVMeMetrics()
}

// DeviceTypeScraper defines type-specific behavior for scraping EBS or Instance Store metrics.
type DeviceTypeScraper interface {
	Model() string
	DeviceType() string
	Identifier(serial string) (string, error)
	SetResourceAttribute(rb *metadata.ResourceBuilder, identifier string)
	RecordMetrics(recordMetric RecordMetricFunc, mb *metadata.MetricsBuilder, ts pcommon.Timestamp, metrics Metrics)
	IsEnabled(m *metadata.MetricsConfig) bool
	ParseRawData(data []byte) (Metrics, error)
}
