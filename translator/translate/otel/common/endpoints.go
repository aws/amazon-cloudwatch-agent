// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common //nolint:revive // existing package name

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws/endpoints"
)

// CloudWatch OTLP endpoint batch limits.
// See: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-OTLPEndpoint.html
const (
	MaxMetricsPerRequest = 1000
	MaxLogsPerRequest    = 10000
	MaxSpansPerRequest   = 10000
	BatchTimeout         = 15 * time.Second
)

func ServiceEndpoint(service, region, path string) string {
	partition, _ := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region)
	dnsSuffix := partition.DNSSuffix()
	if dnsSuffix == "" {
		dnsSuffix = "amazonaws.com"
	}
	return fmt.Sprintf("https://%s.%s.%s%s", service, region, dnsSuffix, path)
}
