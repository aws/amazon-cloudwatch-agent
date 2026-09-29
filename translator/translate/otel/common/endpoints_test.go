// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common //nolint:revive // existing package name

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestServiceEndpoint(t *testing.T) {
	testCases := map[string]struct {
		service string
		region  string
		path    string
		want    string
	}{
		"StandardPartition": {
			service: "monitoring",
			region:  "us-east-1",
			path:    "/v1/metrics",
			want:    "https://monitoring.us-east-1.amazonaws.com/v1/metrics",
		},
		"GovCloudPartition": {
			service: "monitoring",
			region:  "us-gov-west-1",
			path:    "/v1/metrics",
			want:    "https://monitoring.us-gov-west-1.amazonaws.com/v1/metrics",
		},
		"LogsStandardPartition": {
			service: "logs",
			region:  "us-east-1",
			path:    "/v1/logs",
			want:    "https://logs.us-east-1.amazonaws.com/v1/logs",
		},
		"LogsGovCloudPartition": {
			service: "logs",
			region:  "us-gov-west-1",
			path:    "/v1/logs",
			want:    "https://logs.us-gov-west-1.amazonaws.com/v1/logs",
		},
		"ChinaPartition": {
			service: "logs",
			region:  "cn-north-1",
			path:    "/v1/logs",
			want:    "https://logs.cn-north-1.amazonaws.com.cn/v1/logs",
		},
		"ISOPartition": {
			service: "monitoring",
			region:  "us-iso-east-1",
			path:    "/v1/metrics",
			want:    "https://monitoring.us-iso-east-1.c2s.ic.gov/v1/metrics",
		},
		"SovereignPartition": {
			service: "monitoring",
			region:  "eusc-de-east-1",
			path:    "/v1/metrics",
			want:    "https://monitoring.eusc-de-east-1.amazonaws.eu/v1/metrics",
		},
		"UnknownRegionUsesDefaultSuffix": {
			service: "logs",
			region:  "unknown-region",
			path:    "/v1/logs",
			want:    "https://logs.unknown-region.amazonaws.com/v1/logs",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := ServiceEndpoint(tc.service, tc.region, tc.path)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestNormalizeEndpointURL(t *testing.T) {
	testCases := map[string]struct {
		input string
		want  string
	}{
		"DocumentedVPCEndpoint": {
			input: "vpce-0123456789abcdef0-abcdefgh.logs.us-east-1.vpce.amazonaws.com",
			want:  "https://vpce-0123456789abcdef0-abcdefgh.logs.us-east-1.vpce.amazonaws.com",
		},
		"BareServiceHostname": {
			input: "logs.us-east-1.amazonaws.com",
			want:  "https://logs.us-east-1.amazonaws.com",
		},
		"HostWithPort": {
			input: "127.0.0.1:8080",
			want:  "https://127.0.0.1:8080",
		},
		"HostWithPath": {
			input: "example.com/prefix",
			want:  "https://example.com/prefix",
		},
		"HostWithTrailingSlash": {
			input: "example.com/",
			want:  "https://example.com/",
		},
		"PreservesHTTPS": {
			input: "https://logs.us-east-1.amazonaws.com",
			want:  "https://logs.us-east-1.amazonaws.com",
		},
		"PreservesHTTP": {
			input: "http://127.0.0.1:8080",
			want:  "http://127.0.0.1:8080",
		},
		"PreservesHTTPWithPath": {
			input: "http://127.0.0.1:8080/prefix/",
			want:  "http://127.0.0.1:8080/prefix/",
		},
		"PreservesUppercaseScheme": {
			input: "HTTPS://example.com",
			want:  "HTTPS://example.com",
		},
		"TrimsWhitespace": {
			input: "  https://example.com  ",
			want:  "https://example.com",
		},
		"TrimsWhitespaceThenAddsScheme": {
			input: "\texample.com\n",
			want:  "https://example.com",
		},
		"Empty": {
			input: "",
			want:  "",
		},
		"WhitespaceOnly": {
			input: "   ",
			want:  "",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, NormalizeEndpointURL(tc.input))
		})
	}
}

func TestGetEndpointOverride(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"logs": map[string]any{
			"endpoint_override": "vpce-abc.logs.us-east-1.vpce.amazonaws.com",
		},
		"metrics": map[string]any{
			"endpoint_override": "http://localhost:4566",
		},
	})

	got, ok := GetEndpointOverride(conf, ConfigKey(LogsKey, EndpointOverrideKey))
	assert.True(t, ok)
	assert.Equal(t, "https://vpce-abc.logs.us-east-1.vpce.amazonaws.com", got)

	got, ok = GetEndpointOverride(conf, ConfigKey(MetricsKey, EndpointOverrideKey))
	assert.True(t, ok)
	assert.Equal(t, "http://localhost:4566", got)

	got, ok = GetEndpointOverride(conf, ConfigKey(TracesKey, EndpointOverrideKey))
	assert.False(t, ok)
	assert.Equal(t, "", got)
}
