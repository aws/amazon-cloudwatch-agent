// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common //nolint:revive // existing package name

import (
	"fmt"
	"time"

	override "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"
)

// CloudWatch OTLP endpoint batch limits.
// See: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-OTLPEndpoint.html
const (
	MaxMetricsPerRequest = 1000
	MaxLogsPerRequest    = 10000
	MaxSpansPerRequest   = 10000
	BatchTimeout         = 30 * time.Second
	// MetricsBatchTimeout is the batch flush interval for the shared opentelemetry
	// metrics pipeline.
	MetricsBatchTimeout = 10 * time.Second
)

// ServiceEndpoint builds the regional endpoint for an AWS service using the DNS suffix
// of the region's partition, defaulting to the classic partition suffix.
func ServiceEndpoint(service, region, path string) string {
	dnsSuffix := override.GetPartitionDNSSuffix(region)
	if dnsSuffix == "" {
		dnsSuffix = "amazonaws.com"
	}
	return fmt.Sprintf("https://%s.%s.%s%s", service, region, dnsSuffix, path)
}
