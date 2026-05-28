// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := ServiceEndpoint(tc.service, tc.region, tc.path)
			assert.Equal(t, tc.want, got)
		})
	}
}
