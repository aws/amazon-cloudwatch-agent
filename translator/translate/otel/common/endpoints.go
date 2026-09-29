// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common //nolint:revive // existing package name

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	override "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"
	"go.opentelemetry.io/collector/confmap"
)

// urlSchemeRegexp matches a leading URL scheme (RFC 3986 section 3.1) followed by "://".
var urlSchemeRegexp = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.\-]*://`)

// NormalizeEndpointURL prepares an endpoint override for use as an SDK v2
// BaseEndpoint. Surrounding whitespace is trimmed and "https://" is prepended
// when the value has no scheme, matching SDK v1's endpoints.AddScheme. An empty
// value or one that already has a scheme is returned as-is.
func NormalizeEndpointURL(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" || urlSchemeRegexp.MatchString(endpoint) {
		return endpoint
	}
	return "https://" + endpoint
}

// GetEndpointOverride reads the endpoint override string at key from conf and
// returns it normalized via NormalizeEndpointURL. ok is false when the key is
// unset.
func GetEndpointOverride(conf *confmap.Conf, key string) (string, bool) {
	endpoint, ok := GetString(conf, key)
	if !ok {
		return "", false
	}
	return NormalizeEndpointURL(endpoint), true
}

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
